//+build integration

package main_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/gke"
	"github.com/weaveworks/service/kubectl-service/grpc"
)

func TestGetPods(t *testing.T) {
	client := newClient(t)
	defer client.Close()
	kubeCfgYAML := newKubeConfigYAML(t)

	resp, err := client.RunKubectlCmd(context.Background(), &grpc.KubectlRequest{
		Version:    "1.8.6",
		Kubeconfig: kubeCfgYAML,
		Args:       []string{"get", "pods", "--all-namespaces"},
	})
	assert.NoError(t, err)
	// 1.8.6 is packaged with the current version of kubectl-service, hence it was selected to run the provided command:
	assert.Regexp(t, "Dry run: /kubectl/1\\.8\\.6 \\[--kubeconfig=/tmp/kubeconfig[0-9]+ get pods --all-namespaces\\]", resp.Output)

	resp, err = client.RunKubectlCmd(context.Background(), &grpc.KubectlRequest{
		Version:    "1.8.4-gke.0",
		Kubeconfig: kubeCfgYAML,
		Args:       []string{"get", "pods", "--all-namespaces"},
	})
	assert.NoError(t, err)
	// "1.8.4-gke.0"'s closest match is 1.8.6 which is packaged with the current version of kubectl-service, hence it was selected to run the provided command:
	assert.Regexp(t, "Dry run: /kubectl/1\\.8\\.6 \\[--kubeconfig=/tmp/kubeconfig[0-9]+ get pods --all-namespaces\\]", resp.Output)

	resp, err = client.RunKubectlCmd(context.Background(), &grpc.KubectlRequest{
		Version:    "2.0.0-gke.0",
		Kubeconfig: kubeCfgYAML,
		Args:       []string{"get", "pods", "--all-namespaces"},
	})
	assert.NoError(t, err)
	// "2.0.0-gke.0" has no compatible version packaged in the current version of kubectl-service, hence it defaulted to "latest" to run the provided command:
	assert.Regexp(t, "Dry run: /kubectl/latest \\[--kubeconfig=/tmp/kubeconfig[0-9]+ get pods --all-namespaces\\]", resp.Output)
}

func newClient(t *testing.T) *grpc.Client {
	cfg := grpc.Config{HostPort: "kubectl-service.weave.local:4772"}
	client, err := grpc.NewClient(cfg)
	assert.NoError(t, err)
	return client
}

func newKubeConfigYAML(t *testing.T) []byte {
	kubeCfg := gke.NewKubeConfig("dev", "192.168.0.1", "admin", "p4$$w0rd", "fa1c3ce1371f1ca7e")
	kubeCfgYAML, err := kubeCfg.Marshal()
	assert.NoError(t, err)
	return kubeCfgYAML
}
