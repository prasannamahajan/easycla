package metrics

import (
	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/metrics"
	v1Metrics "github.com/communitybridge/easycla/cla-backend-go/metrics"
	"github.com/go-openapi/runtime/middleware"
)

// Configure setups handlers on api with service
func Configure(api *operations.EasyclaAPI, service v1Metrics.Service) {
	api.MetricsGetClaManagerDistributionHandler = metrics.GetClaManagerDistributionHandlerFunc(
		func(params metrics.GetClaManagerDistributionParams, user *auth.User) middleware.Responder {
			result, err := service.GetCLAManagerDistribution()
			if err != nil {
				return metrics.NewGetClaManagerDistributionBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewGetClaManagerDistributionOK().WithPayload(*result)
		})

	api.MetricsGetTotalCountHandler = metrics.GetTotalCountHandlerFunc(
		func(params metrics.GetTotalCountParams, user *auth.User) middleware.Responder {
			result, err := service.GetTotalCountMetrics()
			if err != nil {
				return metrics.NewGetTotalCountBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewGetTotalCountOK().WithPayload(*result)
		})

	api.MetricsGetCompanyMetricHandler = metrics.GetCompanyMetricHandlerFunc(
		func(params metrics.GetCompanyMetricParams, user *auth.User) middleware.Responder {
			result, err := service.GetCompanyMetric(params.CompanyID)
			if err != nil {
				return metrics.NewGetCompanyMetricBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewGetCompanyMetricOK().WithPayload(*result)
		})

	api.MetricsGetProjectMetricHandler = metrics.GetProjectMetricHandlerFunc(
		func(params metrics.GetProjectMetricParams, user *auth.User) middleware.Responder {
			result, err := service.GetProjectMetric(params.ProjectID, params.IDType)
			if err != nil {
				if err.Error() == "metric not found" {
					return metrics.NewGetProjectMetricNotFound()
				}
				return metrics.NewGetProjectMetricBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewGetProjectMetricOK().WithPayload(*result)
		})

	api.MetricsGetTopCompaniesHandler = metrics.GetTopCompaniesHandlerFunc(
		func(params metrics.GetTopCompaniesParams, user *auth.User) middleware.Responder {
			result, err := service.GetTopCompanies()
			if err != nil {
				return metrics.NewGetTopCompaniesBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewGetTopCompaniesOK().WithPayload(*result)
		})

	api.MetricsGetTopProjectsHandler = metrics.GetTopProjectsHandlerFunc(
		func(params metrics.GetTopProjectsParams, user *auth.User) middleware.Responder {
			result, err := service.GetTopProjects()
			if err != nil {
				return metrics.NewGetTopProjectsBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewGetTopProjectsOK().WithPayload(*result)
		})

	api.MetricsListProjectMetricsHandler = metrics.ListProjectMetricsHandlerFunc(
		func(params metrics.ListProjectMetricsParams, user *auth.User) middleware.Responder {
			result, err := service.ListProjectMetrics(params.PageSize, params.NextKey)
			if err != nil {
				return metrics.NewListProjectMetricsBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewListProjectMetricsOK().WithPayload(*result)
		})
	api.MetricsListCompanyProjectMetricsHandler = metrics.ListCompanyProjectMetricsHandlerFunc(
		func(params metrics.ListCompanyProjectMetricsParams, user *auth.User) middleware.Responder {
			result, err := service.ListCompanyProjectMetrics(params.CompanyID)
			if err != nil {
				return metrics.NewListCompanyProjectMetricsBadRequest().WithPayload(errorResponse(err))
			}
			return metrics.NewListCompanyProjectMetricsOK().WithPayload(*result)
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
