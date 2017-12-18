package gke

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/context"

	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"

	log "github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuthScopeCloudPlatform is the Google OAuth scope required for the client in this package to work.
const OAuthScopeCloudPlatform = container.CloudPlatformScope

const (
	active              = "ACTIVE"
	up                  = "UP"
	ok                  = "200"
	internalServerError = "500"
)

// Prometheus metrics for GKE API client.
var clientRequestCollector = instrument.NewHistogramCollectorFromOpts(prometheus.HistogramOpts{
	Namespace: "google",
	Subsystem: "gke_client",
	Name:      "request_duration_seconds",
	Help:      "Response time of Google Kubernetes Engine API requests.",
	Buckets:   prometheus.DefBuckets,
})

func init() {
	clientRequestCollector.Register()
}

// Client is the interface for clients to the Kubernetes Engine (and other related) API(s).
type Client interface {
	ListProjects(ctx context.Context) ([]*cloudresourcemanager.Project, error)
	ListZones(ctx context.Context, projectID string) ([]*compute.Zone, error)
	ListClusters(ctx context.Context) ([]*Cluster, error)
	ListClustersForProject(ctx context.Context, projectID string) ([]*Cluster, error)
	ListClustersForProjectAndZone(ctx context.Context, projectID, zone string) ([]*Cluster, error)
	GetCluster(ctx context.Context, projectID, zone, clusterID string) (*Cluster, error)
}

// Cluster groups identifiers for a Google Kubernetes Engine cluster, and details on the cluster itself.
type Cluster struct {
	ProjectID string             `json:"projectId,omitempty"`
	Zone      string             `json:"zone,omitempty"`
	Cluster   *container.Cluster `json:"cluster,omitempty"`
}

// DefaultClient provides access to Google Kubernetes Engine API
type DefaultClient struct {
	computeService   *compute.Service
	resMgrService    *cloudresourcemanager.Service
	containerService *container.Service
}

// NewClientFromConfig returns a Client accessing the Kubernetes Engine API.
// It uses Google OAuth2 for authentication and authorisation, and
// requires following OAuth scope:
// https://www.googleapis.com/auth/cloud-platform
func NewClientFromConfig(serviceAccountKeyFile string) (*DefaultClient, error) {
	jsonKey, err := ioutil.ReadFile(serviceAccountKeyFile)
	if err != nil {
		return nil, err
	}
	jwtConf, err := google.JWTConfigFromJSON(jsonKey, OAuthScopeCloudPlatform)
	if err != nil {
		return nil, err
	}
	oauthClient := jwtConf.Client(context.Background())
	return newClient(oauthClient)
}

// NewClientFromToken returns a Client accessing the Kubernetes Engine API and others.
func NewClientFromToken(token *oauth2.Token) (*DefaultClient, error) {
	tokenSource := oauth2.StaticTokenSource(token)
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	return newClient(oauthClient)
}

func newClient(oauthClient *http.Client) (*DefaultClient, error) {
	computeService, err := compute.New(oauthClient)
	if err != nil {
		log.Errorf("Failed to create Compute Engine API client: %v", err)
		return nil, err
	}
	resMgrService, err := cloudresourcemanager.New(oauthClient)
	if err != nil {
		log.Errorf("Failed to create Cloud Resource Manager API client: %v", err)
		return nil, err
	}
	containerService, err := container.New(oauthClient)
	if err != nil {
		log.Errorf("Failed to create Kubernetes Engine API client: %v", err)
		return nil, err
	}
	return &DefaultClient{
		computeService:   computeService,
		resMgrService:    resMgrService,
		containerService: containerService,
	}, nil
}

// ListProjects lists currently active projects.
// See also: https://cloud.google.com/resource-manager/reference/rest/v1/projects/list
func (c DefaultClient) ListProjects(ctx context.Context) ([]*cloudresourcemanager.Project, error) {
	method := "GET /projects"
	start := time.Now()
	clientRequestCollector.Before(method, start)

	var projects []*cloudresourcemanager.Project
	req := c.resMgrService.Projects.List()
	if err := req.Pages(ctx, func(page *cloudresourcemanager.ListProjectsResponse) error {
		for _, project := range page.Projects {
			if project.LifecycleState == active {
				projects = append(projects, project)
			}
		}
		return nil
	}); err != nil {
		log.Warnf("Failed to list projects: %v", err)
		clientRequestCollector.After(method, internalServerError, start)
		return nil, err
	}
	clientRequestCollector.After(method, ok, start)
	return projects, nil
}

// ListZones lists available zones for the provided project.
// See also: https://cloud.google.com/compute/docs/reference/latest/zones/list
func (c DefaultClient) ListZones(ctx context.Context, projectID string) ([]*compute.Zone, error) {
	method := "GET /projects/{project}/zones"
	start := time.Now()
	clientRequestCollector.Before(method, start)

	var zones []*compute.Zone
	req := c.computeService.Zones.List(projectID)
	if err := req.Pages(ctx, func(page *compute.ZoneList) error {
		for _, zone := range page.Items {
			if zone.Status == up {
				zones = append(zones, zone)
			}
		}
		return nil
	}); err != nil {
		log.Warnf("Failed to list zones: %v", err)
		clientRequestCollector.After(method, internalServerError, start)
		return nil, err
	}
	clientRequestCollector.After(method, ok, start)
	return zones, nil
}

// ListClusters lists all GKE clusters across all available projects.
// N.B.: it needs to send several requests to GCP's APIs:
// - 1 to get all projects for the configured user, then
// - n to get all the zones for the provided projects (done in parallel, using several go routines), and then
// - m to get the GKE clusters for each zone (done in parallel, using several go routines)
func (c DefaultClient) ListClusters(ctx context.Context) ([]*Cluster, error) {
	method := "GET /projects/all/zones/all/clusters"
	start := time.Now()
	clientRequestCollector.Before(method, start)

	projects, err := c.ListProjects(ctx)
	if err != nil {
		clientRequestCollector.After(method, internalServerError, start)
		return nil, err
	}
	results := make(chan *result)
	for _, project := range projects {
		go func(projectID string) {
			if clusters, err := c.ListClustersForProject(ctx, projectID); err != nil {
				results <- &result{Error: err}
			} else {
				results <- &result{Clusters: clusters}
			}
		}(project.ProjectId)
	}
	var clusters []*Cluster
	errors := []error{}
	for i := 0; i < len(projects); i++ {
		result := <-results
		if result.Error != nil {
			errors = append(errors, result.Error)
		} else {
			clusters = append(clusters, result.Clusters...)
		}
	}
	if len(clusters) == 0 && len(errors) > 1 {
		clientRequestCollector.After(method, internalServerError, start)
	} else {
		clientRequestCollector.After(method, ok, start)
	}
	return clusters, nil
}

// ListClustersForProject lists all GKE clusters for the provided project.
// N.B.: it needs to send several requests to GCP's APIs:
// - 1 to get all zones for the provided project, and then
// - n to get the GKE clusters for each zone (done in parallel, using several go routines)
func (c DefaultClient) ListClustersForProject(ctx context.Context, projectID string) ([]*Cluster, error) {
	method := "GET /projects/{project}/zones/all/clusters"
	start := time.Now()
	clientRequestCollector.Before(method, start)

	zones, err := c.ListZones(ctx, projectID)
	if err != nil {
		clientRequestCollector.After(method, internalServerError, start)
		return nil, err
	}
	results := make(chan *result)
	for _, zone := range zones {
		go func(zoneName string) {
			if clusters, err := c.ListClustersForProjectAndZone(ctx, projectID, zoneName); err != nil {
				log.Warnf("Failed to list clusters: %v", err)
				results <- &result{Error: err}
			} else {
				results <- &result{Clusters: clusters}
			}
		}(zone.Name)
	}
	var clusters []*Cluster
	errors := []error{}
	for i := 0; i < len(zones); i++ {
		result := <-results
		if result.Error != nil {
			errors = append(errors, result.Error)
		} else {
			clusters = append(clusters, result.Clusters...)
		}
	}
	if len(clusters) == 0 && len(errors) > 1 {
		clientRequestCollector.After(method, internalServerError, start)
	} else {
		clientRequestCollector.After(method, ok, start)
	}
	return clusters, nil
}

type result struct {
	Clusters []*Cluster
	Error    error
}

// ListClustersForProjectAndZone lists all GKE clusters for the provided project and zone.
// See also: https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.zones.clusters/list
func (c DefaultClient) ListClustersForProjectAndZone(ctx context.Context, projectID, zone string) ([]*Cluster, error) {
	method := fmt.Sprintf("GET /projects/{project}/zones/%v/clusters", zone)
	start := time.Now()
	clientRequestCollector.Before(method, start)
	resp, err := c.containerService.Projects.Zones.Clusters.List(projectID, zone).Context(ctx).Do()
	if err != nil {
		clientRequestCollector.After(method, internalServerError, start)
		return nil, err
	}
	clientRequestCollector.After(method, strconv.Itoa(resp.HTTPStatusCode), start)
	clusters := []*Cluster{}
	for _, cluster := range resp.Clusters {
		clusters = append(clusters, &Cluster{ProjectID: projectID, Zone: zone, Cluster: cluster})
	}
	return clusters, err
}

// GetCluster gets metadata for the provided project, zone and GKE cluster.
// See also: https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.zones.clusters/get
func (c DefaultClient) GetCluster(ctx context.Context, projectID, zone, clusterID string) (*Cluster, error) {
	method := fmt.Sprintf("GET /projects/{project}/zones/%v/clusters/{cluster}", zone)
	start := time.Now()
	clientRequestCollector.Before(method, start)
	resp, err := c.containerService.Projects.Zones.Clusters.Get(projectID, zone, clusterID).Context(ctx).Do()
	if err != nil {
		clientRequestCollector.After(method, internalServerError, start)
		return nil, err
	}
	clientRequestCollector.After(method, strconv.Itoa(resp.HTTPStatusCode), start)
	return &Cluster{ProjectID: projectID, Zone: zone, Cluster: resp}, nil
}

// NoOpClient is a no-op implementation of GKEClient.
// This implementation is mostly useful for testing.
type NoOpClient struct{}

// ListProjects returns an arbitrary list of one project, and a nil error.
func (c NoOpClient) ListProjects(ctx context.Context) ([]*cloudresourcemanager.Project, error) {
	return []*cloudresourcemanager.Project{
		{
			CreateTime:     "2017-12-18T17:09:58.399Z",
			LifecycleState: "ACTIVE",
			Name:           "gke-integration",
			ProjectId:      "gke-integration",
			ProjectNumber:  335754924270,
		},
	}, nil
}

// ListZones returns an arbitrary list of one zone, and a nil error.
func (c NoOpClient) ListZones(ctx context.Context, projectID string) ([]*compute.Zone, error) {
	return []*compute.Zone{
		{
			AvailableCpuPlatforms: []string{
				"Intel Skylake",
				"Intel Broadwell",
				"Intel Sandy Bridge",
			},
			CreationTimestamp: "1969-12-31T16:00:00.000-08:00",
			Description:       "us-central1-a",
			Id:                2000,
			Kind:              "compute#zone",
			Name:              "us-central1-a",
			Region:            "https://www.googleapis.com/compute/v1/projects/gke-integration/regions/us-central1",
			SelfLink:          "https://www.googleapis.com/compute/v1/projects/gke-integration/zones/us-central1-a",
			Status:            "UP",
		},
	}, nil
}

// ListClusters returns an arbitrary list of one cluster, and a nil error.
func (c NoOpClient) ListClusters(ctx context.Context) ([]*Cluster, error) {
	return c.ListClustersForProject(ctx, "gke-integration")
}

// ListClustersForProject returns an arbitrary list of one cluster, and a nil error.
func (c NoOpClient) ListClustersForProject(ctx context.Context, projectID string) ([]*Cluster, error) {
	return c.ListClustersForProjectAndZone(ctx, projectID, "us-central1-a")
}

// ListClustersForProjectAndZone returns an arbitrary list of one cluster, and a nil error.
func (c NoOpClient) ListClustersForProjectAndZone(ctx context.Context, projectID, zone string) ([]*Cluster, error) {
	return []*Cluster{SampleCluster(projectID, zone)}, nil
}

// GetCluster returns an arbitrary cluster, and a nil error.
func (c NoOpClient) GetCluster(ctx context.Context, projectID, zone, clusterID string) (*Cluster, error) {
	return SampleCluster(projectID, zone), nil
}

// SampleCluster returns a sample cluster object. This is mostly useful for testing.
func SampleCluster(projectID, zone string) *Cluster {
	return &Cluster{
		ProjectID: projectID,
		Zone:      zone,
		Cluster: &container.Cluster{
			ClusterIpv4Cidr:       "10.16.0.0/14",
			CreateTime:            "2018-01-12T15:06:48+00:00",
			CurrentMasterVersion:  "1.8.5-gke.0",
			CurrentNodeCount:      3,
			CurrentNodeVersion:    "1.8.5-gke.0",
			Description:           "Cluster for GKE integration's development and testing",
			Endpoint:              "35.184.163.242",
			InitialClusterVersion: "1.8.5-gke.0",
			InitialNodeCount:      3,
			Locations:             []string{"us-central1-a"},
			MasterAuth: &container.MasterAuth{
				ClientCertificate:    "<client_certificate>",
				ClientKey:            "<client_key>",
				ClusterCaCertificate: "<cluster_ca_certificate>",
				Password:             "pa$$w0rd",
				Username:             "admin",
			},
			Name:    "gke-integration",
			Network: "default",
			NodeConfig: &container.NodeConfig{
				DiskSizeGb:     100,
				ImageType:      "COS",
				MachineType:    "f1-micro",
				ServiceAccount: "default",
			},
			NodeIpv4CidrSize: 24,
			NodePools: []*container.NodePool{
				{
					Config: &container.NodeConfig{
						DiskSizeGb:     100,
						ImageType:      "COS",
						MachineType:    "f1-micro",
						ServiceAccount: "default",
					},
					InitialNodeCount: 3,
					Name:             "default-pool",
					SelfLink:         "https://container.googleapis.com/v1/projects/gke-integration/zones/us-central1-a/clusters/gke-integration/nodePools/default-pool",
					Status:           "RUNNING",
					Version:          "1.8.5-gke.0",
				},
			},
			ServicesIpv4Cidr: "10.19.240.0/20",
			Status:           "RUNNING",
			Subnetwork:       "default",
			Zone:             "us-central1-a",
		},
	}
}
