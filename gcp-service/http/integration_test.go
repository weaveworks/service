//+build integration

package http_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/gcp-service/service"
)

func TestGetProjects(t *testing.T) {
	resp := get(t, "/api/gcp/users/123456/projects")
	assert.Equal(t, "[\"gke-integration\"]\n", resp)
}

func TestGetClusters(t *testing.T) {
	resp := get(t, "/api/gcp/users/123456/clusters")
	clusters := deserializeJSON(t, resp)
	assert.Equal(t, 1, len(clusters))
	cluster := clusters[0]
	assert.Equal(t, "gke-integration", cluster.ProjectID, "ProjectIDs did not match")
	assert.Equal(t, "gke-integration", cluster.ClusterID, "ClusterIDs did not match")
	assert.Equal(t, "us-central1-a", cluster.Zone)
	assert.Equal(t, "1.8.5-gke.0", cluster.KubernetesVersion)
}

func TestGetClustersForProject(t *testing.T) {
	resp := get(t, "/api/gcp/users/123456/projects/gke-integration/clusters")
	clusters := deserializeJSON(t, resp)
	assert.Equal(t, 1, len(clusters))
	cluster := clusters[0]
	assert.Equal(t, "gke-integration", cluster.ProjectID, "ProjectIDs did not match")
	assert.Equal(t, "gke-integration", cluster.ClusterID, "ClusterIDs did not match")
	assert.Equal(t, "us-central1-a", cluster.Zone)
	assert.Equal(t, "1.8.5-gke.0", cluster.KubernetesVersion)
}

func TestRunKubectlCmd(t *testing.T) {
	resp := post(t, "/api/gcp/users/123456/projects/gke-integration/clusters/gke-integration/zones/us-central1-a/kubectl", []string{"get", "pods", "--all-namespaces"})
	assert.Equal(t, "\"Dry run: kubectl get pods --all-namespaces (1.8.5-gke.0)\"\n", resp)
}

func TestInstallWeaveCloud(t *testing.T) {
	post(t, "/api/gcp/users/123456/projects/gke-integration/clusters/gke-integration/zones/us-central1-a/install", map[string]string{
		"token": "abc123",
	})
}

const baseURL = "http://gcp-service.weave.local"

func get(t *testing.T, endpoint string) string {
	resp, err := http.Get(baseURL + endpoint)
	return read(t, resp, err)
}

func post(t *testing.T, endpoint string, data interface{}) string {
	body, err := json.Marshal(data)
	assert.NoError(t, err)
	resp, err := http.Post(baseURL+endpoint, "application/json", bytes.NewBuffer(body))
	return read(t, resp, err)
}

func read(t *testing.T, resp *http.Response, err error) string {
	assert.NoError(t, err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	return string(body)
}

func deserializeJSON(t *testing.T, jsonStr string) []*service.Cluster {
	var clusters []*service.Cluster
	err := json.Unmarshal([]byte(jsonStr), &clusters)
	assert.NoError(t, err)
	return clusters
}
