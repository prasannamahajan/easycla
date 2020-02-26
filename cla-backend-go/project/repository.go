// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	"errors"
	"fmt"
	"sync"

	"github.com/communitybridge/easycla/cla-backend-go/gen/restapi/operations/project"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// errors
var (
	ErrProjectDoesNotExist = errors.New("project does not exist")
	ErrProjectIDMissing    = errors.New("project id is missing")
)

// Repository defines functions of Project repository
type Repository interface {
	GetMetrics() (*models.ProjectMetrics, error)
	CreateProject(project *models.Project) (*models.Project, error)
	GetProject(projectID string) (*models.Project, error)
	GetExternalProject(projectExternalID string) (*models.Project, error)
	GetProjects(params *project.GetProjectsParams) (*models.Projects, error)
	DeleteProject(projectID string) error
	UpdateProject(projectModel *models.Project) (*models.Project, error)
	buildProjectModel(dbModel DBProjectModel) *models.Project
	buildProjectModels(results *dynamodb.ScanOutput) ([]models.Project, error)
}

// NewDynamoRepository creates instance of project repository
func NewDynamoRepository(awsSession *session.Session, stage string) Repository {
	return &repo{
		dynamoDBClient: dynamodb.New(awsSession),
		stage:          stage,
	}
}

type repo struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
}

// GetMetrics returns the metrics for the projects
func (repo repo) GetMetrics() (*models.ProjectMetrics, error) {
	var out models.ProjectMetrics
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)
	// Do these counts in parallel
	var wg sync.WaitGroup
	wg.Add(2)

	var totalCount int64
	var projects []models.ProjectSimpleModel

	go func(tableName string) {
		defer wg.Done()
		// How many total records do we have - may not be up-to-date as this value is updated only periodically
		describeTableInput := &dynamodb.DescribeTableInput{
			TableName: &tableName,
		}
		describeTableResult, err := repo.dynamoDBClient.DescribeTable(describeTableInput)
		if err != nil {
			log.Warnf("error retrieving total record count, error: %v", err)
		}
		// Meta-data for the response
		totalCount = *describeTableResult.Table.ItemCount
	}(tableName)

	go func() {
		defer wg.Done()

		// Use the last evaluated key to determine if we have more to process
		lastEvaluatedKey := ""

		for ok := true; ok; ok = lastEvaluatedKey != "" {
			projectModels, err := repo.GetProjects(&project.GetProjectsParams{
				PageSize: aws.Int64(50),
				NextKey:  aws.String(lastEvaluatedKey),
			})
			if err != nil {
				log.Warnf("error retrieving projects for metrics, error: %v", err)
			}
			// Convert the full response model to a simple model for metrics
			projects = append(projects, buildSimpleModel(projectModels)...)

			// Save the last evaluated key - use it to determine if we have more to process
			lastEvaluatedKey = projectModels.LastKeyScanned
		}
	}()

	// Wait for the counts to finish
	wg.Wait()

	out.TotalCount = totalCount
	out.Projects = projects
	return &out, nil
}

// buildSimpleModel converts the DB model to a simple response model
func buildSimpleModel(dbProjectsModel *models.Projects) []models.ProjectSimpleModel {
	if dbProjectsModel == nil || dbProjectsModel.Projects == nil {
		return []models.ProjectSimpleModel{}
	}

	var simpleModels []models.ProjectSimpleModel
	for _, dbModel := range dbProjectsModel.Projects {
		simpleModels = append(simpleModels, models.ProjectSimpleModel{
			ProjectName:         dbModel.ProjectName,
			ProjectManagerCount: int64(len(dbModel.ProjectACL)),
		})
	}

	return simpleModels
}

// CreateProject creates a new project
func (repo *repo) CreateProject(projectModel *models.Project) (*models.Project, error) {
	// Generate a new project ID
	projectID, err := uuid.NewV4()
	if err != nil {
		log.Warnf("Unable to generate a UUID for a new project request, error: %v", err)
		return nil, err
	}

	_, currentTimeString := utils.CurrentTime()
	input := &dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{},
		TableName: aws.String(fmt.Sprintf("cla-%s-projects", repo.stage)),
	}

	//var individualDocs []*dynamodb.AttributeValue
	//var corporateDocs []*dynamodb.AttributeValue
	addStringAttribute(input.Item, "project_id", projectID.String())
	addStringAttribute(input.Item, "project_external_id", projectModel.ProjectExternalID)
	addStringAttribute(input.Item, "project_name", projectModel.ProjectName)
	addStringSliceAttribute(input.Item, "project_acl", projectModel.ProjectACL)
	addBooleanAttribute(input.Item, "project_icla_enabled", projectModel.ProjectICLAEnabled)
	addBooleanAttribute(input.Item, "project_ccla_enabled", projectModel.ProjectCCLAEnabled)
	addBooleanAttribute(input.Item, "project_ccla_requires_icla_signature", projectModel.ProjectCCLARequiresICLA)
	// TODO
	//addListAttribute(input.Item, "project_individual_documents", individualDocs)
	// TODO
	//addListAttribute(input.Item, "project_member_documents", corporateDocs)
	addStringAttribute(input.Item, "date_created", currentTimeString)
	addStringAttribute(input.Item, "date_modified", currentTimeString)
	addStringAttribute(input.Item, "version", "v1")

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.Warnf("Unable to create a new project record, error: %v", err)
		return nil, err
	}

	// Re-use the provided model - just update the dynamically assigned values
	projectModel.ProjectID = projectID.String()
	projectModel.DateCreated = currentTimeString
	projectModel.DateModified = currentTimeString
	projectModel.Version = "v1"

	return projectModel, nil
}

// GetProject returns the project model associated for the specified projectID
func (repo *repo) GetProject(projectID string) (*models.Project, error) {
	log.Debugf("GetProject - projectID: %s", projectID)
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)
	// This is the key we want to match
	condition := expression.Key("project_id").Equal(expression.Value(projectID))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for Project query, projectID: %s, error: %v",
			projectID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.Warnf("error retrieving project by projectID: %s, error: %v", projectID, queryErr)
		return nil, queryErr
	}

	// No match, didn't find it
	if *results.Count == 0 {
		// Attempt to lookup via external ID
		return repo.GetExternalProject(projectID)
	}

	// Should only have one result
	if *results.Count > 1 {
		log.Warnf("Project query returned more than one result using projectID: %s", projectID)
	}

	var dbModel DBProjectModel
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &dbModel)
	if err != nil {
		log.Warnf("error unmarshalling db project model, error: %+v", err)
		return nil, err
	}

	// Convert the database model to an API response model
	return repo.buildProjectModel(dbModel), nil
}

// GetExternalProject returns the project model associated for the specified external project ID
func (repo *repo) GetExternalProject(projectExternalID string) (*models.Project, error) {
	log.Debugf("GetExternalProject - projectID: %s", projectExternalID)
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)
	// This is the key we want to match
	condition := expression.Key("project_external_id").Equal(expression.Value(projectExternalID))

	// Use the builder to create the expression
	expr, err := expression.NewBuilder().WithKeyCondition(condition).WithProjection(buildProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for Project query, projectExternalID: %s, error: %v",
			projectExternalID, err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
		IndexName:                 aws.String("external-project-index"),
	}

	// Make the DynamoDB Query API call
	results, queryErr := repo.dynamoDBClient.Query(queryInput)
	if queryErr != nil {
		log.Warnf("error retrieving project by projectExternalID: %s, error: %v", projectExternalID, queryErr)
		return nil, queryErr
	}

	// No match, didn't find it
	if *results.Count == 0 {
		return nil, nil
	}

	// Should only have one result
	if *results.Count > 1 {
		log.Warnf("Project query returned more than one result using projectExternalID: %s", projectExternalID)
	}

	var dbModel DBProjectModel
	err = dynamodbattribute.UnmarshalMap(results.Items[0], &dbModel)
	if err != nil {
		log.Warnf("error unmarshalling db project model, error: %+v", err)
		return nil, err
	}

	// Convert the database model to an API response model
	return repo.buildProjectModel(dbModel), nil
}

// GetProjects queries the database and returns a list of the projects
func (repo *repo) GetProjects(params *project.GetProjectsParams) (*models.Projects, error) {
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)

	// Use the nice builder to create the expression
	expr, err := expression.NewBuilder().WithProjection(buildProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for project scan, error: %v", err)
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}

	// If we have the next key, set the exclusive start key value
	if params.NextKey != nil && *params.NextKey != "" {
		log.Debugf("Received a nextKey, value: %s", *params.NextKey)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: params.NextKey,
			},
		}
	}

	// If we have a page size, set the limit value - make sure it's a positive value
	if params.PageSize != nil && *params.PageSize > 0 {
		log.Debugf("Received a pageSize parameter, value: %d", *params.PageSize)
		// The primary key of the first item that this operation will evaluate.
		// and the query key (if not the same)
		scanInput.Limit = params.PageSize
	} else {
		// Default page size
		*params.PageSize = 50
	}

	var projects []models.Project
	var lastEvaluatedKey string

	// Loop until we have all the records
	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Scan(scanInput)
		if errQuery != nil {
			log.Warnf("error retrieving projects, error: %v", errQuery)
			return nil, errQuery
		}

		// Convert the list of DB models to a list of response models
		projectList, modelErr := repo.buildProjectModels(results)
		if modelErr != nil {
			log.Warnf("error converting project DB model to response model, error: %v",
				modelErr)
			return nil, modelErr
		}

		// Add to the project response models to the list
		projects = append(projects, projectList...)

		if results.LastEvaluatedKey["project_id"] != nil {
			lastEvaluatedKey = *results.LastEvaluatedKey["project_id"].S
			scanInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
				"project_id": {
					S: aws.String(lastEvaluatedKey),
				},
			}
		} else {
			lastEvaluatedKey = ""
		}

		if int64(len(projects)) >= *params.PageSize {
			break
		}
	}

	return &models.Projects{
		LastKeyScanned: lastEvaluatedKey,
		PageSize:       *params.PageSize,
		Projects:       projects,
	}, nil
}

// DeleteProject deletes the project by projectID
func (repo *repo) DeleteProject(projectID string) error {
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)

	existingProject, getErr := repo.GetProject(projectID)
	if getErr != nil {
		log.Warnf("delete - error locating the project id: %s, error: %+v", projectID, getErr)
		return getErr
	}

	if existingProject == nil {
		return ErrProjectDoesNotExist
	}

	var deleteErr error
	// Perform the delete
	_, deleteErr = repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: aws.String(existingProject.ProjectID),
			},
		},
	})

	if deleteErr != nil {
		log.Warnf("Error deleting project with ID : %s, error: %v", projectID, deleteErr)
		return deleteErr
	}

	return nil
}

// UpdateProject updates the project by projectID
func (repo *repo) UpdateProject(projectModel *models.Project) (*models.Project, error) {
	tableName := fmt.Sprintf("cla-%s-projects", repo.stage)

	if projectModel.ProjectID == "" {
		return nil, ErrProjectIDMissing
	}

	existingProject, getErr := repo.GetProject(projectModel.ProjectID)
	if getErr != nil {
		log.Warnf("update - error locating the project id: %s, error: %+v", projectModel.ProjectID, getErr)
		return nil, getErr
	}

	if existingProject == nil {
		return nil, ErrProjectDoesNotExist
	}

	expressionAttributeNames := map[string]*string{}
	expressionAttributeValues := map[string]*dynamodb.AttributeValue{}
	updateExpression := "SET "

	if projectModel.ProjectName != "" {
		log.Debugf("UpdateProject - adding project_name: %s", projectModel.ProjectName)
		expressionAttributeNames["#N"] = aws.String("project_name")
		expressionAttributeValues[":n"] = &dynamodb.AttributeValue{S: aws.String(projectModel.ProjectName)}
		updateExpression = updateExpression + " #N = :n, "
	}
	if projectModel.ProjectACL != nil && len(projectModel.ProjectACL) > 0 {
		log.Debugf("UpdateProject - adding project_acl: %s", projectModel.ProjectACL)
		expressionAttributeNames["#A"] = aws.String("project_acl")
		expressionAttributeValues[":a"] = &dynamodb.AttributeValue{SS: aws.StringSlice(projectModel.ProjectACL)}
		updateExpression = updateExpression + " #A = :a, "
	}

	log.Debugf("UpdateProject - adding project_icla_enabled: %t", projectModel.ProjectICLAEnabled)
	expressionAttributeNames["#I"] = aws.String("project_icla_enabled")
	expressionAttributeValues[":i"] = &dynamodb.AttributeValue{BOOL: aws.Bool(projectModel.ProjectICLAEnabled)}
	updateExpression = updateExpression + " #I = :i, "

	log.Debugf("UpdateProject - adding project_ccla_enabled: %t", projectModel.ProjectCCLAEnabled)
	expressionAttributeNames["#C"] = aws.String("project_ccla_enabled")
	expressionAttributeValues[":c"] = &dynamodb.AttributeValue{BOOL: aws.Bool(projectModel.ProjectCCLAEnabled)}
	updateExpression = updateExpression + " #C = :c, "

	log.Debugf("UpdateProject - adding project_ccla_requires_icla_signature: %t", projectModel.ProjectCCLARequiresICLA)
	expressionAttributeNames["#CI"] = aws.String("project_ccla_requires_icla_signature")
	expressionAttributeValues[":ci"] = &dynamodb.AttributeValue{BOOL: aws.Bool(projectModel.ProjectCCLARequiresICLA)}
	updateExpression = updateExpression + " #CI = :ci, "

	_, currentTimeString := utils.CurrentTime()
	log.Debugf("UpdateProject - adding date_modified: %s", currentTimeString)
	expressionAttributeNames["#M"] = aws.String("date_modified")
	expressionAttributeValues[":m"] = &dynamodb.AttributeValue{S: aws.String(currentTimeString)}
	updateExpression = updateExpression + " #M = :m "

	// Assemble the query input parameters
	updateInput := &dynamodb.UpdateItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"project_id": {
				S: aws.String(existingProject.ProjectID),
			},
		},
		ExpressionAttributeNames:  expressionAttributeNames,
		ExpressionAttributeValues: expressionAttributeValues,
		UpdateExpression:          &updateExpression,
		TableName:                 aws.String(tableName),
	}
	//log.Debugf("Update input: %+V", updateInput.GoString())

	// Make the DynamoDB Update API call
	_, updateErr := repo.dynamoDBClient.UpdateItem(updateInput)
	if updateErr != nil {
		log.Warnf("error updating project by projectID: %s, error: %v", projectModel.ProjectID, updateErr)
		return nil, updateErr
	}

	// Read the updated record back from the DB and return - probably could
	// just create/update a new model in memory and return it to make it fast,
	// but this approach return exactly what the DB has
	return repo.GetProject(projectModel.ProjectID)
}

// buildProjectModels converts the database response model into an API response data model
func (repo *repo) buildProjectModels(results *dynamodb.ScanOutput) ([]models.Project, error) {
	var projects []models.Project

	// The DB project model
	var dbProjects []DBProjectModel

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &dbProjects)
	if err != nil {
		log.Warnf("error unmarshalling projects from database, error: %v", err)
		return nil, err
	}

	for _, dbProject := range dbProjects {
		projects = append(projects, models.Project{
			ProjectID:               dbProject.ProjectID,
			ProjectExternalID:       dbProject.ProjectExternalID,
			ProjectName:             dbProject.ProjectName,
			ProjectACL:              dbProject.ProjectACL,
			ProjectCCLAEnabled:      dbProject.ProjectCclaEnabled,
			ProjectICLAEnabled:      dbProject.ProjectIclaEnabled,
			ProjectCCLARequiresICLA: dbProject.ProjectCclaRequiresIclaSignature,
			DateCreated:             dbProject.DateCreated,
			DateModified:            dbProject.DateModified,
			Version:                 dbProject.Version,
		})
	}

	return projects, nil
}

// buildProjectModel maps the database model to the API response model
func (repo *repo) buildProjectModel(dbModel DBProjectModel) *models.Project {
	return &models.Project{
		ProjectID:               dbModel.ProjectID,
		ProjectExternalID:       dbModel.ProjectExternalID,
		ProjectName:             dbModel.ProjectName,
		ProjectACL:              dbModel.ProjectACL,
		ProjectCCLAEnabled:      dbModel.ProjectCclaEnabled,
		ProjectICLAEnabled:      dbModel.ProjectIclaEnabled,
		ProjectCCLARequiresICLA: dbModel.ProjectCclaRequiresIclaSignature,
		DateCreated:             dbModel.DateCreated,
		DateModified:            dbModel.DateModified,
		Version:                 dbModel.Version,
	}
}

// buildProject is a helper function to build a common set of projection/columns for the query
func buildProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("project_id"),
		expression.Name("project_external_id"),
		expression.Name("project_name"),
		expression.Name("project_acl"),
		expression.Name("project_ccla_enabled"),
		expression.Name("project_icla_enabled"),
		expression.Name("project_ccla_requires_icla_signature"),
		expression.Name("date_created"),
		expression.Name("date_modified"),
		expression.Name("version"),
	)
}

// addStringAttribute adds a new string attribute to the existing map
func addStringAttribute(item map[string]*dynamodb.AttributeValue, key string, value string) {
	if value != "" {
		item[key] = &dynamodb.AttributeValue{S: aws.String(value)}
	}
}

// addBooleanAttribute adds a new boolean attribute to the existing map
func addBooleanAttribute(item map[string]*dynamodb.AttributeValue, key string, value bool) {
	item[key] = &dynamodb.AttributeValue{BOOL: aws.Bool(value)}
}

// addStringSliceAttribute adds a new string slice attribute to the existing map
func addStringSliceAttribute(item map[string]*dynamodb.AttributeValue, key string, value []string) {
	item[key] = &dynamodb.AttributeValue{SS: aws.StringSlice(value)}
}

// addListAttribute adds a new list attribute to the existing map
/*
func addListAttribute(item map[string]*dynamodb.AttributeValue, key string, value []*dynamodb.AttributeValue) {
	item[key] = &dynamodb.AttributeValue{L: value}
}
*/
