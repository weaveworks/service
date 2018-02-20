package service

import (
	log "github.com/sirupsen/logrus"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"github.com/weaveworks/launcher/pkg/kubectl"
	"github.com/weaveworks/service/common/gcp/gke"
	"github.com/weaveworks/service/gcp-service/dao"
	pb "github.com/weaveworks/service/kubectl-service/grpc"
)

// Service is our service.
type Service struct {
	UsersClient      dao.UsersClient
	KubectlClient    pb.CloseableKubectlClient
	GKEClientFactory func(*oauth2.Token) (gke.Client, error)
}

// Cluster groups identifiers for a Google Kubernetes Engine cluster and its version, to avoid leaking sensitive information about the cluster itself.
type Cluster struct {
	ProjectID         string `json:"projectId,omitempty"`
	Zone              string `json:"zone,omitempty"`
	ClusterID         string `json:"clusterId,omitempty"`
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`
}

// GetClusters returns all the GKE clusters belonging to the provided user.
func (s Service) GetClusters(ctx context.Context, userID string) ([]*Cluster, error) {
	logger := log.WithField("user_id", userID)
	client, err := s.gkeClientFor(userID)
	if err != nil {
		return nil, err
	}
	clusters, err := client.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	logger.Infof("%v cluster(s) retrieved", len(clusters))
	return summarize(clusters), nil
}

func summarize(clusters []*gke.Cluster) []*Cluster {
	ids := []*Cluster{}
	for _, cluster := range clusters {
		ids = append(ids, &Cluster{
			ProjectID:         cluster.ProjectID,
			Zone:              cluster.Zone,
			ClusterID:         cluster.Cluster.Name,
			KubernetesVersion: cluster.Cluster.CurrentMasterVersion,
		})
	}
	return ids
}

// GetProjects returns all the GCP projects belonging to the provided user.
func (s Service) GetProjects(ctx context.Context, userID string) ([]string, error) {
	logger := log.WithField("user_id", userID)
	client, err := s.gkeClientFor(userID)
	if err != nil {
		return nil, err
	}
	projects, err := client.ListProjects(context.Background())
	if err != nil {
		return nil, err
	}
	var projectIDs []string
	for _, project := range projects {
		projectIDs = append(projectIDs, project.Name)
	}
	logger.Infof("%v project(s) retrieved", len(projectIDs))
	return projectIDs, nil
}

// GetClustersForProject returns all the GKE clusters belonging to the provided user in the specified project.
func (s Service) GetClustersForProject(ctx context.Context, userID, projectID string) ([]*Cluster, error) {
	logger := log.WithFields(log.Fields{"user_id": userID, "project_id": projectID})
	client, err := s.gkeClientFor(userID)
	if err != nil {
		return nil, err
	}
	clusters, err := client.ListClustersForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	logger.Infof("%v cluster(s) retrieved", len(clusters))
	return summarize(clusters), nil
}

// RunKubectlCmd executes the provided kubectl command against the specified cluster.
func (s Service) RunKubectlCmd(ctx context.Context, userID, projectID, zone, clusterID string, args []string) (string, error) {
	logger := log.WithFields(log.Fields{"user_id": userID, "project_id": projectID, "zone": zone, "cluster_id": clusterID})
	client, err := s.gkeClientFor(userID)
	if err != nil {
		return "", err
	}
	cluster, err := client.GetCluster(ctx, projectID, zone, clusterID)
	if err != nil {
		return "", err
	}
	kubeCfg := gke.NewKubeConfig(
		cluster.Cluster.Name,
		cluster.Cluster.Endpoint,
		cluster.Cluster.MasterAuth.Username,
		cluster.Cluster.MasterAuth.Password,
		cluster.Cluster.MasterAuth.ClusterCaCertificate)
	kubeCfgBytes, err := kubeCfg.Marshal()
	if err != nil {
		return "", err
	}
	logger.Infof("Running kubectl command %v", args)
	reply, err := s.KubectlClient.RunKubectlCmd(ctx, &pb.KubectlRequest{
		Version:    cluster.Cluster.CurrentMasterVersion,
		Kubeconfig: kubeCfgBytes,
		Args:       args,
	})
	if err != nil {
		return "", err
	}
	return reply.Output, nil
}

func (s Service) gkeClientFor(userID string) (gke.Client, error) {
	token, err := s.UsersClient.GoogleOAuthToken(userID)
	if err != nil {
		return nil, err
	}
	return s.GKEClientFactory(token)
}

// KubectlServiceClient implements github.com/weaveworks/launcher/pkg/kubectl.Client
type KubectlServiceClient struct {
	Context   context.Context
	Service   Service
	UserID    string
	ProjectID string
	Zone      string
	ClusterID string
}

// Execute implements github.com/weaveworks/launcher/pkg/kubectl.Client
func (k KubectlServiceClient) Execute(args ...string) (string, error) {
	return k.Service.RunKubectlCmd(k.Context, k.UserID, k.ProjectID, k.Zone, k.ClusterID, args)
}

// InstallWeaveCloud executes the provided kubectl command against the specified cluster.
func (s Service) InstallWeaveCloud(ctx context.Context, userID, projectID, zone, clusterID, weaveCloudToken string) error {
	// Create client which implements kubectl.Client so we can use our kubectl pkg helpers
	client := KubectlServiceClient{
		Context:   ctx,
		Service:   s,
		UserID:    userID,
		ProjectID: projectID,
		Zone:      zone,
		ClusterID: clusterID,
	}

	// 1. Create weave namespace
	_, err := kubectl.CreateNamespace(client, "weave")
	if err != nil {
		return err
	}

	// 2. Create weave-cloud token secret
	_, err = kubectl.CreateSecretFromLiteral(client, "weave", "weave-cloud", "token", weaveCloudToken, true)
	if err != nil {
		return err
	}

	// 3. Apply agent k8s
	err = kubectl.Apply(client, "https://get.weave.works/k8s/agent.yaml")
	if err != nil {
		return err
	}

	return nil
}
