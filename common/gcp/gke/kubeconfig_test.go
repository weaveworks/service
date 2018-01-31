package gke_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/common/gcp/gke"
)

const KubeConfigYAML = `apiVersion: v1
kind: Config
current-context: dev
contexts:
- name: dev
  context:
    cluster: dev
    user: admin
clusters:
- name: dev
  cluster:
    certificate-authority-data: foo
    server: https://192.168.0.1
users:
- name: admin
  user:
    username: admin
    password: p4$$w0rd
`

func TestUnmarshalAndMarshalKubeConfig(t *testing.T) {
	var kubeCfg gke.KubeConfig
	err := kubeCfg.Unmarshal(KubeConfigYAML)
	assert.NoError(t, err)

	assert.Equal(t, "v1", kubeCfg.APIVersion)
	assert.Equal(t, "dev", kubeCfg.CurrentContext)
	assert.Equal(t, "Config", kubeCfg.Kind)

	assert.Equal(t, 1, len(kubeCfg.Clusters))
	cluster := kubeCfg.Clusters[0]
	assert.Equal(t, "dev", cluster.Name)
	assert.Equal(t, "https://192.168.0.1", cluster.Cluster.Server)
	assert.Equal(t, "foo", cluster.Cluster.CertificateAuthorityData)

	assert.Equal(t, 1, len(kubeCfg.Contexts))
	context := kubeCfg.Contexts[0]
	assert.Equal(t, "dev", context.Name)
	assert.Equal(t, "dev", context.Context.Cluster)
	assert.Equal(t, "admin", context.Context.User)

	assert.Equal(t, 1, len(kubeCfg.Users))
	user := kubeCfg.Users[0]
	assert.Equal(t, "admin", user.Name)
	assert.Equal(t, "admin", user.User.Username)
	assert.Equal(t, "p4$$w0rd", user.User.Password)

	bytes, err := kubeCfg.Marshal()
	assert.NoError(t, err)
	assert.Equal(t, KubeConfigYAML, string(bytes))
}

func TestCreateViaConstructorAndMarshalKubeConfig(t *testing.T) {
	kubeCfg := gke.NewKubeConfig("dev", "192.168.0.1", "admin", "p4$$w0rd", "foo")
	bytes, err := kubeCfg.Marshal()
	assert.NoError(t, err)
	assert.Equal(t, KubeConfigYAML, string(bytes))
}

func TestCreateManuallyAndMarshalKubeConfig(t *testing.T) {
	kubeCfg := gke.KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "dev",
		Contexts: []gke.ContextItem{
			{
				Name: "dev",
				Context: gke.Context{
					Cluster: "dev",
					User:    "admin",
				},
			},
		},
		Clusters: []gke.ClusterItem{
			{
				Name: "dev",
				Cluster: gke.ClusterCfg{
					CertificateAuthorityData: "foo",
					Server: "https://192.168.0.1",
				},
			},
		},
		Users: []gke.UserItem{
			{
				Name: "admin",
				User: gke.User{
					Username: "admin",
					Password: "p4$$w0rd",
				},
			},
		},
	}
	bytes, err := kubeCfg.Marshal()
	assert.NoError(t, err)
	assert.Equal(t, KubeConfigYAML, string(bytes))
}
