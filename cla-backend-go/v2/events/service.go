package events

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
)

// constants
const (
	DefaultPageSize = 10
)

// errors
var (
	ErrProjectNotFound = errors.New("project not found")
)

type service struct {
	v1EventService events.Service
}

type Service interface {
	GetRecentEventsForCompanyProject(companyID string, projectSFID string, pageSize *int64) (*models.EventList, error)
}

func NewService(v1EventsService events.Service) Service {
	return &service{v1EventService: v1EventsService}
}

func (s *service) GetRecentEventsForCompanyProject(companyID string, projectSFID string, pageSize *int64) (*models.EventList, error) {
	if pageSize == nil {
		pageSize = aws.Int64(DefaultPageSize)
	}
	// Get all projects under projectSFID
	psc := v2ProjectService.GetClient()
	projectSFIDs := utils.NewStringSet()
	projectDetails, err := psc.GetProject(projectSFID)
	if err != nil {
		log.Error("GetRecentEventsForCompanyProject : unable to get project details", err)
		if strings.Contains(err.Error(), "not found") {
			return nil, ErrProjectNotFound
		}
		return nil, err
	}
	projectSFIDs.Add(projectSFID)
	for _, subProject := range projectDetails.Projects {
		projectSFIDs.Add(subProject.ID)
	}
	// Parallely call all the projectSFID for events
	rchan := make(chan *v1Models.EventList)
	wg := &sync.WaitGroup{}
	wg.Add(projectSFIDs.Length())
	go func() {
		wg.Wait()
		close(rchan)
	}()
	log.Warnf("project SFID list: %v", projectSFIDs.List())
	for _, id := range projectSFIDs.List() {
		go func(swg *sync.WaitGroup, pid string, resultChan chan *v1Models.EventList) {
			defer wg.Done()
			log.Debugf("querying events for projectSFID: %s, company: %s", pid, companyID)
			result, err := s.v1EventService.GetRecentEventsForCompanyProject(companyID, pid, pageSize)
			if err != nil {
				log.Warnf("Unable to get events for projectSFID: %s, company: %s", pid, companyID)
				return
			}
			resultChan <- result
		}(wg, id, rchan)
	}
	result := &v1Models.EventList{}

	// Receive the events
	for events := range rchan {
		if events != nil {
			result.Events = append(result.Events, events.Events...)
		}
	}

	// Sort events by time descending
	sort.Slice(result.Events, func(i, j int) bool {
		return result.Events[i].EventTimeEpoch > result.Events[j].EventTimeEpoch
	})

	// return maximum pageSize result
	returnResult := *pageSize
	if returnResult > int64(len(result.Events)) {
		returnResult = int64(len(result.Events))
	}
	result.Events = result.Events[:returnResult]
	return v2EventList(result)
}
