package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/metrics"
	"github.com/sanity-io/litter"

	"github.com/communitybridge/easycla/cla-backend-go/signatures"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	Insert = "INSERT"
	Modify = "MODIFY"
	Remove = "REMOVE"
)

var awsSession = session.Must(session.NewSession(&aws.Config{}))
var metricsRepo metrics.Repository
var stage string

var functions map[string]map[string]([]func(string, events.DynamoDBEventRecord) error)

func init() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	metricsRepo = metrics.NewRepository(awsSession, stage)
	functions = make(map[string]map[string]([]func(string, events.DynamoDBEventRecord) error))
	signaturesTable := fmt.Sprintf("cla-%s-signatures", stage)
	registerCallback(signaturesTable, Modify, AddCompanyProjectCount)
}

func UnmarshalStreamImage(attribute map[string]events.DynamoDBAttributeValue, out interface{}) error {

	dbAttrMap := make(map[string]*dynamodb.AttributeValue)

	for k, v := range attribute {

		var dbAttr dynamodb.AttributeValue

		bytes, marshalErr := v.MarshalJSON()
		if marshalErr != nil {
			return marshalErr
		}

		err := json.Unmarshal(bytes, &dbAttr)
		if err != nil {
			return err
		}
		dbAttrMap[k] = &dbAttr
	}

	return dynamodbattribute.UnmarshalMap(dbAttrMap, out)
}

func AddCompanyProjectCount(stage string, record events.DynamoDBEventRecord) error {
	if record.Change.StreamViewType != string(events.DynamoDBStreamViewTypeNewAndOldImages) {
		return nil
	}
	var newSignature signatures.ItemSignature
	var oldSignature signatures.ItemSignature
	err := UnmarshalStreamImage(record.Change.NewImage, &newSignature)
	if err != nil {
		return err
	}
	err = UnmarshalStreamImage(record.Change.OldImage, &oldSignature)
	if err != nil {
		return err
	}
	if newSignature.SignatureType == "ccla" &&
		newSignature.SignatureReferenceType == "company" &&
		newSignature.SignatureSigned == true &&
		oldSignature.SignatureSigned == false {
		err := metricsRepo.IncreaseCompanyProjectCount(newSignature.SignatureReferenceID, newSignature.SignatureReferenceName)
		if err != nil {
			return err
		}
	}
	return nil
}

func registerCallback(tableName string, action string, callbackFunc func(string, events.DynamoDBEventRecord) error) {
	c, ok := functions[tableName]
	if !ok {
		functions[tableName] = map[string][]func(string, events.DynamoDBEventRecord) error{
			action: {callbackFunc},
		}
		return
	}
	funcs, ok := c[action]
	if !ok {
		functions[tableName][action] = []func(string, events.DynamoDBEventRecord) error{callbackFunc}
		return
	}
	functions[tableName][action] = append(funcs, callbackFunc)
}

func getTableFromArn(arn string) (string, error) {
	arr := strings.Split(arn, ":")
	if len(arr) < 5 {
		return "", errors.New("cannot recognize arn format. arn = " + arn)
	}
	arr2 := strings.Split(arr[5], "/")
	if len(arr2) < 2 {
		return "", errors.New("cannot recognize arn format. arn = " + arn)
	}
	return arr2[1], nil
}

func handleEvent(stage string, event events.DynamoDBEventRecord) error {
	tableName, err := getTableFromArn(event.EventSourceArn)
	if err != nil {
		return err
	}
	if funcMap, ok := functions[tableName]; ok {
		if funcArr, ok := funcMap[event.EventName]; ok {
			for _, callbackFunc := range funcArr {
				fmt.Printf("calling callback for %s: %s\n", tableName, event.EventName)
				err := callbackFunc(stage, event)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func handler(e events.DynamoDBEvent) error {
	for _, event := range e.Records {
		err := handleEvent(stage, event)
		if err != nil {
			log.Printf("error occured while processing event %#v. error = %s\n", event, err)
		}

	}
	return nil
}

func main() {
	cmms, err := metricsRepo.GetCompanyProjectCount("2d72016c-7639-459d-9448-ba77da174e47", 5)
	if err != nil {
		log.Fatal(err)
	}
	litter.Dump(cmms)
	os.Exit(0)
	var e events.DynamoDBEventRecord
	err = json.Unmarshal([]byte(ev), &e)
	if err != nil {
		log.Fatal(err)
	}
	handleEvent(stage, e)
	//lambda.Start(handler)
}

var ev = `{
    "awsRegion": "us-east-1",
    "dynamodb": {
        "ApproximateCreationDateTime": 1583133207,
        "Keys": {
            "signature_id": {
                "S": "8db6d3c2-ee70-4428-9ec1-cbe500ce0061"
            }
        },
        "NewImage": {
            "date_created": {
                "S": "2019-08-06T20:02:00.199863+0000"
            },
            "date_modified": {
                "S": "2019-08-06T20:02:00.199870+0000"
            },
            "domain_whitelist": {
                "L": [
                    {
                        "S": "biarca.net"
                    }
                ]
            },
            "signature_acl": {
                "SS": [
                    "manikantanr",
                    "p.mentee.3"
                ]
            },
            "signature_approved": {
                "BOOL": true
            },
            "signature_callback_url": {
                "S": "https://api.dev.lfcla.com/v2/signed/corporate/ad364d61-bb4e-462c-96cb-9d09c4402474/2d72016c-7639-459d-9448-ba77da174e47"
            },
            "signature_document_major_version": {
                "N": "2"
            },
            "signature_document_minor_version": {
                "N": "0"
            },
            "signature_envelope_id": {
                "S": "17950368-0dfc-4574-91c9-167b788c39ed"
            },
            "signature_id": {
                "S": "8db6d3c2-ee70-4428-9ec1-cbe500ce0061"
            },
            "signature_project_id": {
                "S": "ad364d61-bb4e-462c-96cb-9d09c4402474"
            },
            "signature_reference_id": {
                "S": "2d72016c-7639-459d-9448-ba77da174e47"
            },
            "signature_reference_name": {
                "S": "Biarca"
            },
            "signature_reference_name_lower": {
                "S": "biarca"
            },
            "signature_reference_type": {
                "S": "company"
            },
            "signature_return_url": {
                "S": "https://corporate.dev.lfcla.com/#/company/2d72016c-7639-459d-9448-ba77da174e47"
            },
            "signature_sign_url": {
                "S": "https://demo.docusign.net/Signing/MTRedeem/v1?slt=eyJ0eXAiOiJNVCIsImFsZyI6IlJTMjU2Iiwia2lkIjoiNjgxODVmZjEtNGU1MS00Y2U5LWFmMWMtNjg5ODEyMjAzMzE3In0.AQUAAAABAAMABwAAE5bJqxrXSAgAgNrNe6wa10gYAAEAAAAAAAAAIQB5AwAAeyJUb2tlbklkIjoiYzMyZjkwZDAtOGYyMy00MzZhLWFlNTctN2Q1MDgxZGRkZjAxIiwiU3ViamVjdCI6bnVsbCwiU3ViamVjdFR5cGUiOm51bGwsIkV4cGlyYXRpb24iOiIyMDE5LTA4LTA2VDIwOjI3OjIxKzAwOjAwIiwiSXNzdWVkQXQiOiIyMDE5LTA4LTA2VDIwOjIyOjIyLjM3NDExNjkrMDA6MDAiLCJSZXNvdXJjZUlkIjoiMTc5NTAzNjgtMGRmYy00NTc0LTkxYzktMTY3Yjc4OGMzOWVkIiwiTGFiZWwiOm51bGwsIlNpdGVJZCI6bnVsbCwiUmVzb3VyY2VzIjoie1wiRW52ZWxvcGVJZFwiOlwiMTc5NTAzNjgtMGRmYy00NTc0LTkxYzktMTY3Yjc4OGMzOWVkXCIsXCJBY3RvclVzZXJJZFwiOlwiZGYxZGU1OGMtOTA1Yi00YzExLWJhZmMtYWI4OTFmN2I2ZGMyXCIsXCJSZWNpcGllbnRJZFwiOlwiYzI5ZjQ1MzEtYWU4YS00NmVhLWEwZjItYzgzZmFlYTY3OThhXCIsXCJGYWtlUXVlcnlTdHJpbmdcIjpcInQ9OGYyNjlmMzktM2E0NS00OTc0LTk1MjktZWY5MWYyYjQzYzE0XCJ9IiwiT0F1dGhTdGF0ZSI6bnVsbCwiVG9rZW5UeXBlIjoxLCJBbGxvd1JlY2lwaWVudFVzZXJzIjpmYWxzZSwiQXVkaWVuY2UiOiIyNWUwOTM5OC0wMzQ0LTQ5MGMtOGU1My0zYWIyY2E1NjI3YmYiLCJTY29wZXMiOm51bGwsIlJlZGlyZWN0VXJpIjoiaHR0cHM6Ly9kZW1vLmRvY3VzaWduLm5ldC9TaWduaW5nL1N0YXJ0SW5TZXNzaW9uLmFzcHgiLCJIYXNoQWxnb3JpdGhtIjowLCJIYXNoU2FsdCI6bnVsbCwiSGFzaFJvdW5kcyI6MCwiVG9rZW5TZWNyZXRIYXNoIjpudWxsLCJUb2tlblN0YXR1cyI6MCwiRXh0ZXJuYWxDbGFpbXNSZXF1ZXN0ZWQiOm51bGwsIlRyYW5zYWN0aW9uSWQiOm51bGwsIlRyYW5zYWN0aW9uRXZlbnRDYWxsYmFja1VybCI6bnVsbCwiSXNTaW5nbGVVc2UiOmZhbHNlfQ.YbhcyTaQe7Hxe0p9Vxi_LC0sLDOQUTnsdzbc_U5CWH1tgycD8D_7MtmDUEXXQYGZNInKtzEe026LG4yIPHMVS10pYz7Z-QqE_7ZH2HenNvrLF7MIUyKabSsxFdupXNq_weGOnw4KBYqY2pxHR3HmMFoQVBUioO5bt-hMa_DWFLEkyXkjiaFBYBo2458HzZYPtwDVyjtsA6BbR3GDC3-AT3oPTOo3mAE9laWA1tVoOAJd2hqpBOsAPIQyZnOJTURG19_rMMOQW4ZCxPfch9kb_Y0icaAKWYX9dFJllkrfwYP98F6pm_Oht7wibs_zz6hFRieDqPwaS2GYa7tt6YJrZg"
            },
            "signature_signed": {
                "BOOL": true
            },
            "signature_type": {
                "S": "ccla"
            },
            "version": {
                "S": "v1"
            }
        },
        "OldImage": {
            "date_created": {
                "S": "2019-08-06T20:02:00.199863+0000"
            },
            "date_modified": {
                "S": "2019-08-06T20:02:00.199870+0000"
            },
            "domain_whitelist": {
                "L": [
                    {
                        "S": "biarca.net"
                    }
                ]
            },
            "signature_acl": {
                "SS": [
                    "manikantanr",
                    "p.mentee.3",
                    "prasanna.mahajan"
                ]
            },
            "signature_approved": {
                "BOOL": true
            },
            "signature_callback_url": {
                "S": "https://api.dev.lfcla.com/v2/signed/corporate/ad364d61-bb4e-462c-96cb-9d09c4402474/2d72016c-7639-459d-9448-ba77da174e47"
            },
            "signature_document_major_version": {
                "N": "2"
            },
            "signature_document_minor_version": {
                "N": "0"
            },
            "signature_envelope_id": {
                "S": "17950368-0dfc-4574-91c9-167b788c39ed"
            },
            "signature_id": {
                "S": "8db6d3c2-ee70-4428-9ec1-cbe500ce0061"
            },
            "signature_project_id": {
                "S": "ad364d61-bb4e-462c-96cb-9d09c4402474"
            },
            "signature_reference_id": {
                "S": "2d72016c-7639-459d-9448-ba77da174e47"
            },
            "signature_reference_name": {
                "S": "Biarca"
            },
            "signature_reference_name_lower": {
                "S": "biarca"
            },
            "signature_reference_type": {
                "S": "company"
            },
            "signature_return_url": {
                "S": "https://corporate.dev.lfcla.com/#/company/2d72016c-7639-459d-9448-ba77da174e47"
            },
            "signature_sign_url": {
                "S": "https://demo.docusign.net/Signing/MTRedeem/v1?slt=eyJ0eXAiOiJNVCIsImFsZyI6IlJTMjU2Iiwia2lkIjoiNjgxODVmZjEtNGU1MS00Y2U5LWFmMWMtNjg5ODEyMjAzMzE3In0.AQUAAAABAAMABwAAE5bJqxrXSAgAgNrNe6wa10gYAAEAAAAAAAAAIQB5AwAAeyJUb2tlbklkIjoiYzMyZjkwZDAtOGYyMy00MzZhLWFlNTctN2Q1MDgxZGRkZjAxIiwiU3ViamVjdCI6bnVsbCwiU3ViamVjdFR5cGUiOm51bGwsIkV4cGlyYXRpb24iOiIyMDE5LTA4LTA2VDIwOjI3OjIxKzAwOjAwIiwiSXNzdWVkQXQiOiIyMDE5LTA4LTA2VDIwOjIyOjIyLjM3NDExNjkrMDA6MDAiLCJSZXNvdXJjZUlkIjoiMTc5NTAzNjgtMGRmYy00NTc0LTkxYzktMTY3Yjc4OGMzOWVkIiwiTGFiZWwiOm51bGwsIlNpdGVJZCI6bnVsbCwiUmVzb3VyY2VzIjoie1wiRW52ZWxvcGVJZFwiOlwiMTc5NTAzNjgtMGRmYy00NTc0LTkxYzktMTY3Yjc4OGMzOWVkXCIsXCJBY3RvclVzZXJJZFwiOlwiZGYxZGU1OGMtOTA1Yi00YzExLWJhZmMtYWI4OTFmN2I2ZGMyXCIsXCJSZWNpcGllbnRJZFwiOlwiYzI5ZjQ1MzEtYWU4YS00NmVhLWEwZjItYzgzZmFlYTY3OThhXCIsXCJGYWtlUXVlcnlTdHJpbmdcIjpcInQ9OGYyNjlmMzktM2E0NS00OTc0LTk1MjktZWY5MWYyYjQzYzE0XCJ9IiwiT0F1dGhTdGF0ZSI6bnVsbCwiVG9rZW5UeXBlIjoxLCJBbGxvd1JlY2lwaWVudFVzZXJzIjpmYWxzZSwiQXVkaWVuY2UiOiIyNWUwOTM5OC0wMzQ0LTQ5MGMtOGU1My0zYWIyY2E1NjI3YmYiLCJTY29wZXMiOm51bGwsIlJlZGlyZWN0VXJpIjoiaHR0cHM6Ly9kZW1vLmRvY3VzaWduLm5ldC9TaWduaW5nL1N0YXJ0SW5TZXNzaW9uLmFzcHgiLCJIYXNoQWxnb3JpdGhtIjowLCJIYXNoU2FsdCI6bnVsbCwiSGFzaFJvdW5kcyI6MCwiVG9rZW5TZWNyZXRIYXNoIjpudWxsLCJUb2tlblN0YXR1cyI6MCwiRXh0ZXJuYWxDbGFpbXNSZXF1ZXN0ZWQiOm51bGwsIlRyYW5zYWN0aW9uSWQiOm51bGwsIlRyYW5zYWN0aW9uRXZlbnRDYWxsYmFja1VybCI6bnVsbCwiSXNTaW5nbGVVc2UiOmZhbHNlfQ.YbhcyTaQe7Hxe0p9Vxi_LC0sLDOQUTnsdzbc_U5CWH1tgycD8D_7MtmDUEXXQYGZNInKtzEe026LG4yIPHMVS10pYz7Z-QqE_7ZH2HenNvrLF7MIUyKabSsxFdupXNq_weGOnw4KBYqY2pxHR3HmMFoQVBUioO5bt-hMa_DWFLEkyXkjiaFBYBo2458HzZYPtwDVyjtsA6BbR3GDC3-AT3oPTOo3mAE9laWA1tVoOAJd2hqpBOsAPIQyZnOJTURG19_rMMOQW4ZCxPfch9kb_Y0icaAKWYX9dFJllkrfwYP98F6pm_Oht7wibs_zz6hFRieDqPwaS2GYa7tt6YJrZg"
            },
            "signature_signed": {
                "BOOL": false
            },
            "signature_type": {
                "S": "ccla"
            },
            "version": {
                "S": "v1"
            }
        },
        "SequenceNumber": "318199300000000016167613689",
        "SizeBytes": 5242,
        "StreamViewType": "NEW_AND_OLD_IMAGES"
    },
    "eventID": "3ffe9d1cb44fc3dc52239e563f12a9d3",
    "eventName": "MODIFY",
    "eventSource": "aws:dynamodb",
    "eventVersion": "1.1",
    "eventSourceARN": "arn:aws:dynamodb:us-east-1:395594542180:table/cla-dev-signatures/stream/2020-03-02T05:49:58.779"
}`
