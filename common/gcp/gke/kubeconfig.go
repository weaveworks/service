package gke

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// KubeConfig represents a configuration file for kubectl.
type KubeConfig struct {
	APIVersion     string        `yaml:"apiVersion"`
	Kind           string        `yaml:"kind"`
	CurrentContext string        `yaml:"current-context"`
	Contexts       []ContextItem `yaml:"contexts"`
	Clusters       []ClusterItem `yaml:"clusters"`
	Users          []UserItem    `yaml:"users"`
}

// ContextItem represents an element in the "contexts" field of a KubeConfig.
type ContextItem struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

// Context represents a context in a KubeConfig.
type Context struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

// ClusterItem represents an element in the "clusters" field of a KubeConfig.
type ClusterItem struct {
	Name    string     `yaml:"name"`
	Cluster ClusterCfg `yaml:"cluster"`
}

// ClusterCfg represents a cluster in a KubeConfig.
type ClusterCfg struct {
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
	Server                   string `yaml:"server"`
}

// UserItem represents an element in the "users" field of a KubeConfig.
type UserItem struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

// User represents an user in a KubeConfig.
type User struct {
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// Unmarshal unmarshals the provided YAML string into this instance of KubeConfig.
func (kcfg *KubeConfig) Unmarshal(yamlStr string) error {
	return yaml.Unmarshal([]byte(yamlStr), kcfg)
}

// Marshal marshals this instance of KubeConfig.
func (kcfg KubeConfig) Marshal() ([]byte, error) {
	return yaml.Marshal(kcfg)
}

// NewKubeConfig helps constructing a new KubeConfig.
func NewKubeConfig(clusterName, endpoint, username, password, clusterCaCertificate string) *KubeConfig {
	return &KubeConfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: clusterName,
		Contexts: []ContextItem{
			{
				Name: clusterName,
				Context: Context{
					Cluster: clusterName,
					User:    username,
				},
			},
		},
		Clusters: []ClusterItem{
			{
				Name: clusterName,
				Cluster: ClusterCfg{
					CertificateAuthorityData: clusterCaCertificate,
					Server: fmt.Sprintf("https://%v", endpoint),
				},
			},
		},
		Users: []UserItem{
			{
				Name: username,
				User: User{
					Username: username,
					Password: password,
				},
			},
		},
	}
}
