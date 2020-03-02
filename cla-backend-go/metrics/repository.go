package metrics

import (
	"errors"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/labstack/gommon/log"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

var (
	ErrDoesNotExist = errors.New("requested item does not exist")
)

const (
	CompanyMetricObject = "company"
)

type Repository interface {
	GetCompanyMetricModel(companyID string) (*CompanyMetricsModel, error)
	CreateCompanyMetricModel(companyID string, companyName string, projectCount int64) error
	IncreaseCompanyProjectCount(companyID string, companyName string) error

	GetCompanyProjectCount(companyID string, top int) ([]CompanyMetricsModel, error)
}

type repo struct {
	tableName      string
	dynamoDBClient *dynamodb.DynamoDB
}

func NewRepository(awsSession *session.Session, stage string) Repository {
	return &repo{
		dynamoDBClient: dynamodb.New(awsSession),
		tableName:      "cla-dev-stream-test",
	}
}

type CompanyMetricsModel struct {
	ID           string `dynamodbav:"id"`
	ObjectType   string `dynamodbav:"object_type"`
	ProjectCount int64  `dynamodbav:"project_count"`
	CompanyName  string `dynamodbav:"company_name"`
}

func (repo *repo) GetCompanyMetricModel(companyID string) (*CompanyMetricsModel, error) {
	id := "company#" + companyID

	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(result.Item) == 0 {
		return nil, ErrDoesNotExist
	}
	var out CompanyMetricsModel
	err = dynamodbattribute.UnmarshalMap(result.Item, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (repo *repo) CreateCompanyMetricModel(companyID string, companyName string, projectCount int64) error {
	input := &dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{},
		TableName: aws.String(repo.tableName),
	}
	id := "company#" + companyID
	utils.AddStringAttribute(input.Item, "id", id)
	utils.AddStringAttribute(input.Item, "company_name", companyName)
	utils.AddStringAttribute(input.Item, "object_type", CompanyMetricObject)
	utils.AddNumberAttribute(input.Item, "project_count", projectCount)
	_, err := repo.dynamoDBClient.PutItem(input)
	return err
}

func (repo *repo) IncreaseCompanyProjectCount(companyID string, companyName string) error {
	cmm, err := repo.GetCompanyMetricModel(companyID)
	if err != nil {
		if err == ErrDoesNotExist {
			return repo.CreateCompanyMetricModel(companyID, companyName, 1)
		}
		return err
	}
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(cmm.ID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("project_count"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":n": {
				N: aws.String(strconv.FormatInt(cmm.ProjectCount+1, 10)),
			},
		},
		UpdateExpression: aws.String("SET #N = :n"),
	}
	_, err = repo.dynamoDBClient.UpdateItem(input)
	return err
}

func (repo *repo) GetCompanyProjectCount(companyID string, top int) ([]CompanyMetricsModel, error) {

	filter := expression.Name("object_type").Equal(expression.Value(CompanyMetricObject))

	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(buildCompanyMetricProjection()).Build()
	if err != nil {
		log.Warnf("error building expression for company metric scan, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.tableName),
	}

	var companyMetrics []CompanyMetricsModel
	for {
		results, errQuery := repo.dynamoDBClient.Scan(scanInput)
		if errQuery != nil {
			log.Warnf("error retrieving signatures, error: %v", errQuery)
			return nil, errQuery
		}

		var companyMetricsTmp []CompanyMetricsModel

		err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &companyMetricsTmp)
		if err != nil {
			log.Warnf("error unmarshalling company metrics from database. error: %v", err)
			return nil, err
		}
		companyMetrics = append(companyMetrics, companyMetricsTmp...)

		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	id := "company#" + companyID
	sort.Slice(companyMetrics, func(i, j int) bool {
		if companyMetrics[i].ID == id {
			return true
		}
		if companyMetrics[j].ID == id {
			return false
		}
		return companyMetrics[i].ProjectCount > companyMetrics[j].ProjectCount
	})
	if len(companyMetrics) > top {
		return companyMetrics[:top], nil
	}
	return companyMetrics, nil
}

func buildCompanyMetricProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("id"),
		expression.Name("project_count"),
		expression.Name("object_type"),
		expression.Name("company_name"),
	)
}
