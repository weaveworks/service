package utils

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// TODO move to common

// AWSConfigFromURLString parses URL string and returns *aws.Config, prefix and error.
// *aws.Config contains:
// - user/password from url.User.Username(), url.User.Password()
// - region from url.Host
// - prefix from url.Path
//  URL examples:
// DYNAMODB_URL = dynamodb://123user:123password@localhost:8090/events
// SQS_URL = sqs://123user:123password@localhost:9324/events
func AWSConfigFromURLString(urlString string) (cfg *aws.Config, prefix string, err error) {
	url, err := url.Parse(urlString)
	if err != nil {
		return nil, "", err
	}
	if url.User == nil {
		return nil, "", fmt.Errorf("Must specify username & password in URL")
	}

	password, _ := url.User.Password()
	creds := credentials.NewStaticCredentials(url.User.Username(), password, "")
	cfg = aws.NewConfig().WithCredentials(creds)

	if strings.Contains(url.Host, "local") {
		cfg.WithEndpoint(fmt.Sprintf("http://%s", url.Host)).WithRegion("dummy")
	} else {
		cfg.WithRegion(url.Host)
	}
	prefix = strings.TrimPrefix(url.Path, "/")

	return cfg, prefix, nil
}
