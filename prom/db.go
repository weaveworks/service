package main

import (
	"flag"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

const (
	hashKey  = "h"
	rangeKey = "r"
	valueKey = "c"

	tableName = "notebooks"
)

// DynamoDBConfig specifies config for a DynamoDB database.
type DynamoDBConfig struct {
	DynamoDB URLValue
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *DynamoDBConfig) RegisterFlags(f *flag.FlagSet) {
	f.Var(&cfg.DynamoDB, "dynamodb.url", "DynamoDB endpoint URL with escaped Key and Secret encoded. "+
		"If only region is specified as a host, proper endpoint will be deduced. Use inmemory:///<table-name> to use a mock in-memory implementation.")
}

// DynamoDBClient provides functions to interact with notebooks in the database
type DynamoDBClient struct {
	DynamoDB dynamodbiface.DynamoDBAPI
}

// GetAllNotebooks returns all notebooks
func (db DynamoDBClient) GetAllNotebooks(userID string) ([]Notebook, error) {
	params := &dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditions: map[string]*dynamodb.Condition{
			hashKey: {
				AttributeValueList: []*dynamodb.AttributeValue{
					{S: aws.String(userID)},
				},
				ComparisonOperator: aws.String(dynamodb.ComparisonOperatorEq),
			},
		},
		ReturnConsumedCapacity: aws.String(dynamodb.ReturnConsumedCapacityTotal),
	}

	resp, err := db.DynamoDB.Query(params)
	if err != nil {
		return nil, err
	}

	notebooks := []Notebook{}
	for _, item := range resp.Items {
		notebook := Notebook{}
		dynamodbattribute.Unmarshal(item[valueKey], notebook)
		notebooks = append(notebooks, notebook)
	}

	return notebooks, nil
}
