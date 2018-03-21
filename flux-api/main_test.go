// +build integration

package main

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"

	"io/ioutil"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/api/v6"
	"github.com/weaveworks/flux/guid"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	httpdaemon "github.com/weaveworks/flux/http/daemon"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/bus/nats"
	"github.com/weaveworks/service/flux-api/db"
	"github.com/weaveworks/service/flux-api/history"
	historysql "github.com/weaveworks/service/flux-api/history/sql"
	httpserver "github.com/weaveworks/service/flux-api/http"
	"github.com/weaveworks/service/flux-api/instance"
	instancedb "github.com/weaveworks/service/flux-api/instance/sql"
	"github.com/weaveworks/service/flux-api/notifications"
	"github.com/weaveworks/service/flux-api/server"
	"github.com/weaveworks/service/flux-api/service"
)

var (
	// server is a test HTTP server used to provide mock API responses.
	ts *httptest.Server

	// Stores information about service configuration (e.g. automation)
	instanceDB instance.ConnectionDB

	// Mux router
	router *mux.Router

	// Mocked out remote platform.
	mockPlatform *remote.MockServer

	// API Client
	apiClient *client.Client
)

const (
	helloWorldSvc = "default/helloworld"
	ver           = "123"
	id            = service.InstanceID("californian-hotel-76")
)

var (
	testPostgres = flag.String("postgres-url", "postgres://postgres@postgres:5432?sslmode=disable", "Postgres connection string")
)

func setup(t *testing.T) {
	u, err := url.Parse(*testPostgres)
	if err != nil {
		t.Fatal(err)
	}
	databaseMigrationsDir, _ := filepath.Abs("db/migrations/postgres")
	var dbDriver string
	{
		db.Migrate(*testPostgres, databaseMigrationsDir)
		dbDriver = u.Scheme
	}

	// Message bus
	messageBus, err := nats.NewMessageBus("nats://nats:4222")
	if err != nil {
		t.Fatal(err)
	}

	imageID, _ := image.ParseRef("quay.io/weaveworks/helloworld:v1")
	mockPlatform = &remote.MockServer{
		ListServicesAnswer: []v6.ControllerStatus{
			{
				ID:     flux.MustParseResourceID(helloWorldSvc),
				Status: "ok",
				Containers: []v6.Container{
					{
						Name: "helloworld",
						Current: image.Info{
							ID: imageID,
						},
					},
				},
			},
			{},
		},
		ListImagesAnswer: []v6.ImageStatus{
			{
				ID: flux.MustParseResourceID(helloWorldSvc),
				Containers: []v6.Container{
					{
						Name: "helloworld",
						Current: image.Info{
							ID: imageID,
						},
					},
				},
			},
			{
				ID: flux.MustParseResourceID("a/another"),
				Containers: []v6.Container{
					{
						Name: "helloworld",
						Current: image.Info{
							ID: imageID,
						},
					},
				},
			},
		},
	}
	done := make(chan error)
	ctx := context.Background()
	messageBus.Subscribe(ctx, id, mockPlatform, done)
	if err := messageBus.AwaitPresence(id, 5*time.Second); err != nil {
		t.Errorf("Timed out waiting for presence of mockPlatform")
	}

	// History
	hDb, _ := historysql.NewSQL(dbDriver, *testPostgres)
	historyDB := history.InstrumentedDB(hDb)

	// Instancer
	db, err := instancedb.New(dbDriver, *testPostgres)
	if err != nil {
		t.Fatal(err)
	}
	instanceDB = instance.InstrumentedDB(db)

	var instancer instance.Instancer
	{
		// Instancer, for the instancing of operations
		instancer = &instance.MultitenantInstancer{
			DB:        instanceDB,
			Connecter: messageBus,
			Logger:    log.NewNopLogger(),
			History:   historyDB,
		}
	}

	// Server
	apiServer := server.New(ver, instancer, instanceDB, messageBus, log.NewNopLogger(), notifications.DefaultURL)
	router = httpserver.NewServiceRouter()
	httpServer := httpserver.NewServer(apiServer, apiServer, apiServer)
	handler := httpServer.MakeHandler(router, log.NewNopLogger())
	handler = addInstanceIDHandler(handler)
	ts = httptest.NewServer(handler)
	apiClient = client.New(http.DefaultClient, router, ts.URL, "")
}

func addInstanceIDHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Add(httpserver.InstanceIDHeaderKey, string(id))
		handler.ServeHTTP(w, r)
	})
}

func teardown() {
	ts.Close()
}

func TestFluxsvc_ListServices(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	// Test ListServices
	svcs, err := apiClient.ListServices(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(svcs) != 2 {
		t.Fatal("Expected there to be two services")
	}
	if svcs[0].ID.String() != helloWorldSvc && svcs[1].ID.String() != helloWorldSvc {
		t.Errorf("Expected one of the services to be %q", helloWorldSvc)
	}

	// Test that `namespace` argument is mandatory
	u, err := transport.MakeURL(ts.URL, router, "ListServices")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should result in 404, but got: %q", resp.Status)
	}
}

// Note that this test will reach out to docker hub to check the images
// associated with alpine
func TestFluxsvc_ListImages(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	// Test ListImages
	imgs, err := apiClient.ListImages(ctx, update.ResourceSpecAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Error("Expected there two sets of images")
	}
	if len(imgs[0].Containers) == 0 && len(imgs[1].Containers) == 0 {
		t.Error("Should have been lots of containers")
	}

	// Test ListImages for specific service
	imgs, err = apiClient.ListImages(ctx, helloWorldSvc)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 2 {
		t.Error("Expected two sets of images")
	}
	if len(imgs[0].Containers) == 0 {
		t.Error("Expected >1 containers")
	}

	// Test that `service` argument is mandatory
	u, err := transport.MakeURL(ts.URL, router, "ListImages")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Request should result in 404, but got: %s", resp.Status)
	}
}

func TestFluxsvc_Release(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	mockPlatform.UpdateManifestsAnswer = job.ID(guid.New())
	mockPlatform.JobStatusAnswer = job.Status{
		StatusString: job.StatusQueued,
	}

	// Test UpdateImages
	spec := update.ReleaseSpec{
		ImageSpec:    "alpine:latest",
		Kind:         "execute",
		ServiceSpecs: []update.ResourceSpec{helloWorldSvc},
	}
	r, err := apiClient.UpdateManifests(ctx, update.Spec{
		Type: update.Images,
		Spec: spec,
	})
	if err != nil {
		t.Fatal(err)
	}
	if r != mockPlatform.UpdateManifestsAnswer {
		t.Errorf("%q != %q", r, mockPlatform.UpdateManifestsAnswer)
	}

	// Test GetRelease
	res, err := apiClient.JobStatus(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusString != job.StatusQueued {
		t.Error("Unexpected job status: " + res.StatusString)
	}

	// Test JobStatus without parameters
	u, _ := transport.MakeURL(ts.URL, router, "UpdateImages", "service", "default/service")
	resp, err := http.Post(u.String(), "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Path should 404, got: %s", resp.Status)
	}
}

func TestFluxsvc_History(t *testing.T) {
	setup(t)
	defer teardown()

	// TODO: determine whether this test should pass. If it should,
	// fix it. If not, delete this as part of #1894.
	// ctx := context.Background()
	// err := apiClient.LogEvent(ctx, event.Event{
	// 	Type: event.EventLock,
	// 	ServiceIDs: []flux.ResourceID{
	// 		flux.MustParseResourceID(helloWorldSvc),
	// 	},
	// 	Message:   "default/helloworld locked.",
	// 	StartedAt: time.Now().UTC(),
	// 	EndedAt:   time.Now().UTC(),
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// var hist []history.Entry
	// err = apiClient.Get(ctx, &hist, "History", "service", helloWorldSvc)
	// if err != nil {
	// 	t.Error(err)
	// } else {
	// 	var hasLock bool
	// 	for _, v := range hist {
	// 		if strings.Contains(v.Data, "Locked") {
	// 			hasLock = true
	// 			break
	// 		}
	// 	}
	// 	if !hasLock {
	// 		t.Error("History hasn't recorded a lock", hist)
	// 	}
	// }

	// Test `service` argument is mandatory
	u, _ := transport.MakeURL(ts.URL, router, "History")
	resp, err := http.Get(u.String())
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Request should result in 404, got: %s", resp.Status)
	}
}

func TestFluxsvc_Status(t *testing.T) {
	setup(t)
	defer teardown()

	ctx := context.Background()

	// Test Status
	var status service.Status
	err := apiClient.Get(ctx, &status, "Status")
	if err != nil {
		t.Fatal(err)
	}
	if status.Fluxsvc.Version != ver {
		t.Fatalf("Expected %q, got %q", ver, status.Fluxsvc.Version)
	}
}

func TestFluxsvc_Ping(t *testing.T) {
	setup(t)
	defer teardown()

	// Test Ping
	u, err := transport.MakeURL(ts.URL, router, httpserver.Ping)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Request should have been ok but got %q, body:\n%q", resp.Status, string(body))
	}
}

func TestFluxsvc_Register(t *testing.T) {
	setup(t)
	defer teardown()

	_, err := httpdaemon.NewUpstream(&http.Client{}, "fluxd/test", "", router, ts.URL, mockPlatform, log.NewNopLogger()) // For ping and for
	if err != nil {
		t.Fatal(err)
	}

	// Test Ping to make sure daemon has registered.
	u, err := transport.MakeURL(ts.URL, router, httpserver.Ping)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatalf("Request should have been ok but got %q, body:\n%q", resp.Status, string(body))
	}
}
