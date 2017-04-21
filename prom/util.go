package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// dynamoClientFromURL creates a new DynamoDB client from a URL.
func dynamoClientFromURL(awsURL *url.URL) (dynamodbiface.DynamoDBAPI, error) {
	if awsURL == nil {
		return nil, fmt.Errorf("no URL specified for DynamoDB")
	}
	config, err := awsConfigFromURL(awsURL)
	if err != nil {
		return nil, err
	}
	return dynamodb.New(session.New(config)), nil
}

// awsConfigFromURL returns AWS config from given URL. It expects escaped AWS Access key ID & Secret Access Key to be
// encoded in the URL. It also expects region specified as a host (letting AWS generate full endpoint) or fully valid
// endpoint with dummy region assumed (e.g for URLs to emulated services).
func awsConfigFromURL(awsURL *url.URL) (*aws.Config, error) {
	if awsURL.User == nil {
		return nil, fmt.Errorf("must specify escaped Access Key & Secret Access in URL")
	}

	password, _ := awsURL.User.Password()
	creds := credentials.NewStaticCredentials(awsURL.User.Username(), password, "")
	config := aws.NewConfig().
		WithCredentials(creds).
		WithMaxRetries(0) // We do our own retries, so we can monitor them
	if strings.Contains(awsURL.Host, ".") {
		return config.WithEndpoint(fmt.Sprintf("http://%s", awsURL.Host)).WithRegion("dummy"), nil
	}

	// Let AWS generate default endpoint based on region passed as a host in URL.
	return config.WithRegion(awsURL.Host), nil
}

// URLValue is a url.URL that can be used as a flag.
type URLValue struct {
	*url.URL
}

// String implements flag.Value
func (v URLValue) String() string {
	if v.URL == nil {
		return ""
	}
	return v.URL.String()
}

// Set implements flag.Value
func (v *URLValue) Set(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	v.URL = u
	return nil
}
