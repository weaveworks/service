package gke_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/gcp/gke"
	"golang.org/x/net/context"
)

func TestListClusters(t *testing.T) {
	// A working service account file is available at:
	// https://github.com/weaveworks/service-conf/blob/e7022942b99bc9912cda864b6dbc2a1b85a38f20/k8s/dev/default/gcp-launcher-secret.yaml#L26
	serviceAccountFilePath := os.Getenv("SERVICE_ACCOUNT_FILEPATH")
	if serviceAccountFilePath == "" {
		t.Skip("No service account JSON file provided. Could not run test.")
	}
	client, err := gke.NewClientFromConfig(serviceAccountFilePath)
	assert.NoError(t, err)
	clusters, err := client.ListClusters(context.Background())
	assert.NoError(t, err)
	assert.NotEmpty(t, clusters)
}
