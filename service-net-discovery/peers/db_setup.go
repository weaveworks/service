package peers

import (
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go/aws"

	awsutils "github.com/weaveworks/common/aws"
)

func awsConfigFromURLString(urlString string) (*aws.Config, string, error) {
	url, err := url.Parse(urlString)
	if err != nil {
		return nil, "", err
	}

	config, err := awsutils.ConfigFromURL(url)
	if err != nil {
		return nil, "", err
	}

	tableName := strings.TrimPrefix(url.Path, "/")

	return config, tableName, nil
}
