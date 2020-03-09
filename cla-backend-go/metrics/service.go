package metrics

import (
	"math"
	"sort"
	"sync"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/restapi/operations/metrics"
	"github.com/communitybridge/easycla/cla-backend-go/project"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/users"
)

// Service interface defines function of Metrics service
type Service interface {
	GetMetrics(params metrics.GetMetricsParams) (*models.Metrics, error)
	GetCLAManagerDistribution(params metrics.GetClaManagerDistributionParams) (*models.ClaManagerDistribution, error)
	GetTotalCountMetrics() (*models.TotalCountMetrics, error)
	GetCompanyMetric(companyID string) (*models.CompanyMetric, error)
	GetProjectMetric(projectID string) (*models.ProjectMetric, error)
	GetTopCompanies() (*models.TopCompanies, error)
}

type service struct {
	userRepo         users.Repository
	companyRepo      company.RepositoryService
	repositoriesRepo repositories.Repository
	signatureRepo    signatures.SignatureRepository
	projectRepo      project.Repository
	metricsRepo      Repository
}

// NewService creates new instance of metrics service
func NewService(
	userRepo users.Repository,
	companyRepo company.RepositoryService,
	repositoriesRepo repositories.Repository,
	signatureRepo signatures.SignatureRepository,
	projectRepo project.Repository,
	metricsRepo Repository,
) Service {
	return &service{
		userRepo:         userRepo,
		companyRepo:      companyRepo,
		repositoriesRepo: repositoriesRepo,
		signatureRepo:    signatureRepo,
		projectRepo:      projectRepo,
		metricsRepo:      metricsRepo,
	}
}

func (s *service) GetMetrics(params metrics.GetMetricsParams) (*models.Metrics, error) {
	var out models.Metrics
	var wg sync.WaitGroup
	var userMetrics *models.UserMetrics
	var signatureMetrics *models.SignatureMetrics
	var companyMetrics *models.CompaniesMetrics
	var repositoriesMetrics *models.RepositoryMetrics
	var projectMetrics *models.ProjectMetrics
	wg.Add(5)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		var err error
		userMetrics, err = s.userRepo.GetMetrics()
		if err != nil {
			log.Warnf("Unable to get user metrics. error = %v", err)
			return
		}
	}(&wg)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		var err error
		signatureMetrics, err = s.signatureRepo.GetMetrics()
		if err != nil {
			log.Warnf("Unable to get signature metrics. error = %v", err)
			return
		}
	}(&wg)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		var err error
		companyMetrics, err = s.companyRepo.GetMetrics()
		if err != nil {
			log.Warnf("Unable to get company metrics. error = %v", err)
			return
		}

	}(&wg)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		var err error
		repositoriesMetrics, err = s.repositoriesRepo.GetMetrics()
		if err != nil {
			log.Warnf("Unable to get repository metrics. error = %v", err)
			return
		}
	}(&wg)

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		var err error
		projectMetrics, err = s.projectRepo.GetMetrics()
		if err != nil {
			log.Warnf("Unable to get project metrics. error = %v", err)
			return
		}
	}(&wg)

	wg.Wait()

	if userMetrics != nil {
		out.Users = *userMetrics
	}
	if signatureMetrics != nil {
		out.Signatures = *signatureMetrics
	}
	if companyMetrics != nil {
		out.Companies = *companyMetrics
	}
	if repositoriesMetrics != nil {
		out.Repositories = *repositoriesMetrics
	}
	if projectMetrics != nil {
		out.Projects = *projectMetrics
	}
	return &out, nil
}

func (s *service) GetCLAManagerDistribution(params metrics.GetClaManagerDistributionParams) (*models.ClaManagerDistribution, error) {
	cmd, err := s.metricsRepo.GetClaManagerDistribution()
	if err != nil {
		return nil, err
	}
	return &models.ClaManagerDistribution{
		FourOrMoreClaManagers: cmd.FourOrMoreClaManager,
		OneClaManager:         cmd.OneClaManager,
		ThreeClaManagers:      cmd.ThreeClaManager,
		TwoClaManagers:        cmd.TwoClaManager,
		CreatedAt:             cmd.CreatedAt,
	}, nil
}

func (s *service) GetTotalCountMetrics() (*models.TotalCountMetrics, error) {
	tmc, err := s.metricsRepo.GetTotalCountMetrics()
	if err != nil {
		return nil, err
	}
	return &models.TotalCountMetrics{
		ClaManagersCount:                  tmc.ClaManagersCount,
		ContributorsCount:                 tmc.ContributorsCount,
		CorporateContributorsCount:        tmc.CorporateContributorsCount,
		CreatedAt:                         tmc.CreatedAt,
		IndividualContributorsCount:       tmc.IndividualContributorsCount,
		CompaniesCount:                    tmc.CompaniesCount,
		ProjectsCount:                     tmc.ProjectsCount,
		RepositoriesCount:                 tmc.RepositoriesCount,
		CompaniesProjectContributionCount: tmc.CompaniesProjectContributionCount,
	}, nil
}

func (s *service) GetCompanyMetric(companyID string) (*models.CompanyMetric, error) {
	cm, err := s.metricsRepo.GetCompanyMetric(companyID)
	if err != nil {
		return nil, err
	}
	return cm.toModel(), nil
}

func (s *service) GetProjectMetric(projectID string) (*models.ProjectMetric, error) {
	cm, err := s.metricsRepo.GetProjectMetric(projectID)
	if err != nil {
		return nil, err
	}
	return &models.ProjectMetric{
		ClaManagersCount:            cm.ClaManagersCount,
		CompaniesCount:              cm.CompaniesCount,
		CorporateContributorsCount:  cm.CorporateContributorsCount,
		CreatedAt:                   cm.CreatedAt,
		ID:                          cm.ID,
		IndividualContributorsCount: cm.IndividualContributorsCount,
		RepositoriesCount:           cm.RepositoriesCount,
		TotalContributorsCount:      cm.TotalContributorsCount,
		ExternalProjectID:           cm.ExternalProjectID,
	}, nil
}

func average(numerator, denominator int64) int64 {
	return int64(math.Round(float64(numerator) / float64(denominator)))
}

func (s *service) GetTopCompanies() (*models.TopCompanies, error) {
	var averageClaManagers, averageCorporateContributors, averageProjects int64
	returnCount := 5
	cmetrics, err := s.metricsRepo.GetCompanyMetrics()
	if err != nil {
		return nil, err
	}
	if len(cmetrics) < returnCount {
		returnCount = len(cmetrics)
	}
	cmByCorporateContributors := make([]*CompanyMetric, len(cmetrics))
	cmByClaManagers := make([]*CompanyMetric, len(cmetrics))
	cmByProjectCount := make([]*CompanyMetric, len(cmetrics))
	copy(cmByCorporateContributors, cmetrics)
	copy(cmByClaManagers, cmetrics)
	copy(cmByProjectCount, cmetrics)

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		tm, err := s.metricsRepo.GetTotalCountMetrics()
		if err != nil {
			log.Warnf("unable to get total count metrics. error = %s", err.Error())
			return
		}
		averageClaManagers = average(tm.ClaManagersCount, tm.CompaniesCount)
		averageCorporateContributors = average(tm.CorporateContributorsCount, tm.CompaniesCount)
		averageProjects = average(tm.CompaniesProjectContributionCount, tm.CompaniesCount)
	}()
	go func() {
		defer wg.Done()
		sort.Slice(cmByProjectCount, func(i, j int) bool {
			return cmByProjectCount[i].ProjectCount > cmByProjectCount[j].ProjectCount
		})
	}()

	go func() {
		defer wg.Done()
		sort.Slice(cmByCorporateContributors, func(i, j int) bool {
			return cmByCorporateContributors[i].CorporateContributorsCount > cmByCorporateContributors[j].CorporateContributorsCount
		})
	}()

	go func() {
		defer wg.Done()
		sort.Slice(cmByClaManagers, func(i, j int) bool {
			return cmByClaManagers[i].ClaManagersCount > cmByClaManagers[j].ClaManagersCount
		})
	}()
	wg.Wait()
	return &models.TopCompanies{
		AverageCompanyClaManagers:           averageClaManagers,
		AverageCompanyCorporateContributors: averageCorporateContributors,
		AverageCompanyProjectCount:          averageProjects,
		TopCompaniesByClaManagers:           companiesToModel(cmByClaManagers[:returnCount]),
		TopCompaniesByCorporateContributors: companiesToModel(cmByCorporateContributors[:returnCount]),
		TopCompaniesByProjectCount:          companiesToModel(cmByProjectCount[:returnCount]),
	}, nil
}
