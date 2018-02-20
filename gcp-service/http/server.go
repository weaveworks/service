package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/weaveworks/launcher/pkg/kubectl"

	"github.com/weaveworks/service/common/render"
	gcprender "github.com/weaveworks/service/gcp-service/render"
	"github.com/weaveworks/service/gcp-service/service"

	"github.com/gorilla/mux"
)

// Server handles HTTP requests/responses, but delegates actual servicing of requests to... Service.
type Server struct {
	Service *service.Service
}

// Path parameters for this HTTP server:
const (
	UserID    = "userID"
	ProjectID = "projectID"
	Zone      = "zone"
	ClusterID = "clusterID"
)

// RegisterRoutes registers the users API HTTP routes to the provided Router.
func (s Server) RegisterRoutes(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.HandlerFunc
	}{
		// Get the specified user's GCP projects:
		{"users_projects", "GET", fmt.Sprintf("/api/gcp/users/{%v}/projects", UserID), s.GetProjects},

		// Get the specified user's GKE clusters:
		{"users_clusters", "GET", fmt.Sprintf("/api/gcp/users/{%v}/clusters", UserID), s.GetClusters},

		// Get the specified user's GKE clusters within the specified GCP project:
		{"users_projects_clusters", "GET", fmt.Sprintf("/api/gcp/users/{%v}/projects/{%v}/clusters", UserID, ProjectID), s.GetClustersForProject},

		// Run the provided kubectl command (as a JSON array in the request's body) to the specified GKE cluster (belonging to the specified user, and existing in the specified zone):
		{"users_projects_clusters_zones_kubectl", "POST", fmt.Sprintf("/api/gcp/users/{%v}/projects/{%v}/clusters/{%v}/zones/{%v}/kubectl", UserID, ProjectID, ClusterID, Zone), s.RunKubectlCmd},

		// Install Weave Cloud to the specified GKE cluster (belonging to the specified user, and existing in the specified zone):
		{"users_projects_clusters_zones_install", "POST", fmt.Sprintf("/api/gcp/users/{%v}/projects/{%v}/clusters/{%v}/zones/{%v}/install", UserID, ProjectID, ClusterID, Zone), s.InstallWeaveCloud},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}

// GetClusters returns all the GKE clusters belonging to the provided user in the specified project.
func (s Server) GetClusters(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)[UserID]
	clusters, err := s.Service.GetClusters(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
	} else {
		render.JSON(w, http.StatusOK, clusters)
	}
}

// GetProjects returns all the GCP projects belonging to the provided user.
func (s Server) GetProjects(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)[UserID]
	projectIDs, err := s.Service.GetProjects(r.Context(), userID)
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
	} else {
		render.JSON(w, http.StatusOK, projectIDs)
	}
}

// GetClustersForProject returns all the GKE clusters belonging to the provided user in the specified project.
func (s Server) GetClustersForProject(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)[UserID]
	projectID := mux.Vars(r)[ProjectID]
	clusters, err := s.Service.GetClustersForProject(r.Context(), userID, projectID)
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
	} else {
		render.JSON(w, http.StatusOK, clusters)
	}
}

// RunKubectlCmd executes the provided kubectl command against the specified cluster.
func (s Server) RunKubectlCmd(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)[UserID]
	projectID := mux.Vars(r)[ProjectID]
	zone := mux.Vars(r)[Zone]
	clusterID := mux.Vars(r)[ClusterID]
	args, err := deserializeStringArray(r.Body)
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
	}
	out, err := s.Service.RunKubectlCmd(r.Context(), userID, projectID, zone, clusterID, args)
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
	} else {
		render.JSON(w, http.StatusOK, out)
	}
}

func deserializeStringArray(body io.ReadCloser) ([]string, error) {
	bytes, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	var strings []string
	err = json.Unmarshal(bytes, &strings)
	if err != nil {
		return nil, err
	}
	return strings, nil
}

// KubectlServiceClient implements kubectl.Client
type KubectlServiceClient struct {
	Context   context.Context
	Service   *service.Service
	UserID    string
	ProjectID string
	Zone      string
	ClusterID string
}

// Execute implements kubectl.Client
func (k KubectlServiceClient) Execute(args ...string) (string, error) {
	return k.Service.RunKubectlCmd(k.Context, k.UserID, k.ProjectID, k.Zone, k.ClusterID, args)
}

// InstallWeaveCloud executes the provided kubectl command against the specified cluster.
func (s Server) InstallWeaveCloud(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)[UserID]
	projectID := mux.Vars(r)[ProjectID]
	zone := mux.Vars(r)[Zone]
	clusterID := mux.Vars(r)[ClusterID]

	var payload struct {
		Token string `json:"token"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	client := KubectlServiceClient{
		Context:   r.Context(),
		Service:   s.Service,
		UserID:    userID,
		ProjectID: projectID,
		Zone:      zone,
		ClusterID: clusterID,
	}

	// 1. Create weave namespace
	_, err := kubectl.CreateNamespace(client, "weave")
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
		return
	}

	// 2. Create weave-cloud token secret
	_, err = kubectl.CreateSecretFromLiteral(client, "weave", "weave-cloud", "token", payload.Token, true)
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
		return
	}

	// 3. Apply agent k8s
	err = kubectl.Apply(client, "https://get.weave.works/k8s/agent.yaml")
	if err != nil {
		render.Error(w, r, err, gcprender.ErrorStatusCode)
		return
	}

	render.JSON(w, http.StatusOK, nil)
}
