//+build integration

package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/gcp-service/grpc"
)

func TestGetProjects(t *testing.T) {
	client := newClient(t)
	defer client.Close()
	reply, err := client.GetProjects(context.Background(), &grpc.ProjectsRequest{
		UserID: "123456",
	})
	assert.NoError(t, err)
	assert.NotNil(t, reply)
	assert.Equal(t, 1, len(reply.ProjectIDs))
	assert.Equal(t, "gke-integration", reply.ProjectIDs[0])
}

var expectedCluster = &grpc.Cluster{
	ProjectID:         "gke-integration",
	ClusterID:         "gke-integration",
	Zone:              "us-central1-a",
	KubernetesVersion: "1.8.5-gke.0",
}

func TestGetClusters(t *testing.T) {
	client := newClient(t)
	defer client.Close()
	reply, err := client.GetClusters(context.Background(), &grpc.ClustersRequest{
		UserID: "123456",
	})
	assert.NoError(t, err)
	assert.NotNil(t, reply)
	assert.Equal(t, 1, len(reply.Clusters))
	assert.Equal(t, expectedCluster, reply.Clusters[0])
}

func TestGetClustersForProject(t *testing.T) {
	client := newClient(t)
	defer client.Close()
	reply, err := client.GetClustersForProject(context.Background(), &grpc.ClustersRequest{
		UserID:    "123456",
		ProjectID: "gke-integration",
	})
	assert.NoError(t, err)
	assert.NotNil(t, reply)
	assert.Equal(t, 1, len(reply.Clusters))
	assert.Equal(t, expectedCluster, reply.Clusters[0])
}

func TestRunKubectlCmd(t *testing.T) {
	client := newClient(t)
	defer client.Close()
	reply, err := client.RunKubectlCmd(context.Background(), &grpc.KubectlCmdRequest{
		UserID:    "123456",
		ProjectID: "gke-integration",
		Zone:      "us-central1-a",
		ClusterID: "gke-integration",
		Args:      []string{"get", "pods", "--all-namespaces"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, reply)
	assert.Equal(t, "Dry run: kubectl get pods --all-namespaces (1.8.5-gke.0)", reply.Output)
}

func TestInstallWeaveCloud(t *testing.T) {
	client := newClient(t)
	defer client.Close()
	reply, err := client.InstallWeaveCloud(context.Background(), &grpc.InstallWeaveCloudRequest{
		UserID:          "123456",
		ProjectID:       "gke-integration",
		Zone:            "us-central1-a",
		ClusterID:       "gke-integration",
		WeaveCloudToken: "abc123",
	})
	assert.NoError(t, err)
	assert.NotNil(t, reply)
}

func newClient(t *testing.T) *grpc.Client {
	cfg := grpc.Config{HostPort: "gcp-service.weave.local:4772"}
	client, err := grpc.NewClient(cfg)
	assert.NoError(t, err)
	return client
}
