package grpc

import (
	"golang.org/x/net/context"

	"github.com/weaveworks/service/gcp-service/service"
)

// Server implements GCPServer, handles gRPC requests/responses, but delegates actual servicing of requests to... Service.
type Server struct {
	Service *service.Service
}

// GetClusters returns all the GKE clusters belonging to the provided user.
func (s Server) GetClusters(ctx context.Context, req *ClustersRequest) (*ClustersReply, error) {
	allClusters, err := s.Service.GetClusters(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	var clusters []*Cluster
	for _, cluster := range allClusters {
		clusters = append(clusters, toProtobuf(cluster))
	}
	return &ClustersReply{Clusters: clusters}, nil
}

func toProtobuf(cluster *service.Cluster) *Cluster {
	return &Cluster{
		ProjectID:         cluster.ProjectID,
		Zone:              cluster.Zone,
		ClusterID:         cluster.ClusterID,
		KubernetesVersion: cluster.KubernetesVersion,
	}
}

// GetProjects returns all the GCP projects belonging to the provided user.
func (s Server) GetProjects(ctx context.Context, req *ProjectsRequest) (*ProjectsReply, error) {
	projectIDs, err := s.Service.GetProjects(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	return &ProjectsReply{ProjectIDs: projectIDs}, nil
}

// GetClustersForProject returns all the GKE clusters belonging to the provided user in the specified project.
func (s Server) GetClustersForProject(ctx context.Context, req *ClustersRequest) (*ClustersReply, error) {
	allClusters, err := s.Service.GetClustersForProject(ctx, req.UserID, req.ProjectID)
	if err != nil {
		return nil, err
	}
	var clusters []*Cluster
	for _, cluster := range allClusters {
		clusters = append(clusters, toProtobuf(cluster))
	}
	return &ClustersReply{Clusters: clusters}, nil
}

// RunKubectlCmd executes the provided kubectl command against the specified cluster.
func (s Server) RunKubectlCmd(ctx context.Context, req *KubectlCmdRequest) (*KubectlCmdReply, error) {
	out, err := s.Service.RunKubectlCmd(ctx, req.UserID, req.ProjectID, req.Zone, req.ClusterID, req.Args)
	if err != nil {
		return nil, err
	}
	return &KubectlCmdReply{Output: out}, nil
}

// InstallWeaveCloud installs Weave Cloud against the specified cluster.
func (s Server) InstallWeaveCloud(ctx context.Context, req *InstallWeaveCloudRequest) (*InstallWeaveCloudReply, error) {
	err := s.Service.InstallWeaveCloud(ctx, req.UserID, req.ProjectID, req.Zone, req.ClusterID, req.Token)
	if err != nil {
		return nil, err
	}
	return &InstallWeaveCloudReply{}, nil
}
