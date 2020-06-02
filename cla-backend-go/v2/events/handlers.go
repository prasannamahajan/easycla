package events

import (
	"fmt"

	"github.com/labstack/gommon/log"

	"github.com/LF-Engineering/lfx-kit/auth"
	v1Company "github.com/communitybridge/easycla/cla-backend-go/company"
	v1Events "github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/events"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
	"github.com/jinzhu/copier"
)

func v2EventList(eventList *v1Models.EventList) (*models.EventList, error) {
	var dst models.EventList
	err := copier.Copy(&dst, eventList)
	if err != nil {
		return nil, err
	}
	return &dst, nil
}
func isUserAuthorizedForOrganization(user *auth.User, externalCompanyID string) bool {
	if !user.Admin {
		if !user.Allowed || !user.IsUserAuthorizedForOrganizationScope(externalCompanyID) {
			return false
		}
	}
	return true
}

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service v1Events.Service, v1CompanyRepo v1Company.IRepository, v2EventsService Service) {
	api.EventsGetRecentEventsHandler = events.GetRecentEventsHandlerFunc(
		func(params events.GetRecentEventsParams, user *auth.User) middleware.Responder {
			result, err := service.GetRecentEvents(params.PageSize)
			if err != nil {
				return events.NewGetRecentEventsBadRequest().WithPayload(errorResponse(err))
			}
			resp, err := v2EventList(result)
			if err != nil {
				return events.NewGetRecentEventsInternalServerError().WithPayload(errorResponse(err))
			}
			return events.NewGetRecentEventsOK().WithPayload(resp)
		})

	api.EventsGetRecentCompanyProjectEventsHandler = events.GetRecentCompanyProjectEventsHandlerFunc(
		func(params events.GetRecentCompanyProjectEventsParams, authUser *auth.User) middleware.Responder {
			utils.SetAuthUserProperties(authUser, params.XUSERNAME, params.XEMAIL)
			if !isUserAuthorizedForOrganization(authUser, params.CompanySFID) {
				msg := fmt.Sprintf("user %s does not have access of %s", utils.StringValue(params.XUSERNAME), params.CompanySFID)
				log.Warn(msg)
				return events.NewGetRecentCompanyProjectEventsForbidden().WithPayload(&models.ErrorResponse{
					Code:    "403",
					Message: msg,
				})
			}
			comp, err := v1CompanyRepo.GetCompanyByExternalID(params.CompanySFID)
			if err != nil {
				if err == v1Company.ErrCompanyDoesNotExist {
					return events.NewGetRecentCompanyProjectEventsNotFound().WithPayload(&models.ErrorResponse{
						Code:    "404",
						Message: fmt.Sprintf("company %s not exist in cla database", params.CompanySFID),
					})
				}
				return events.NewGetRecentCompanyProjectEventsInternalServerError().WithPayload(errorResponse(err))
			}
			result, err := v2EventsService.GetRecentEventsForCompanyProject(comp.CompanyID, params.ProjectSFID, params.PageSize)
			if err != nil {
				if err == ErrProjectNotFound {
					msg := fmt.Sprintf("invalid projectSFID: %s", params.ProjectSFID)
					log.Warn(msg)
					return events.NewGetRecentCompanyProjectEventsBadRequest().WithPayload(&models.ErrorResponse{
						Code:    "400",
						Message: msg,
					})
				}
				return events.NewGetRecentCompanyProjectEventsInternalServerError().WithPayload(errorResponse(err))
			}
			return events.NewGetRecentCompanyProjectEventsOK().WithPayload(result)
		})
}

type codedResponse interface {
	Code() string
}

func errorResponse(err error) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}

	return &e
}
