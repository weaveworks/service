package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	gh "github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// client is the GitHub client being tested.
	client *gh.Client

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server
)

// setup sets up a test HTTP server along with a github.Client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	// github client configured to use test server
	client = gh.NewClient(nil)
	url, _ := url.Parse(server.URL)
	client.BaseURL = url
	client.UploadURL = url
}

// teardown closes the test HTTP server.
func teardown() {
	server.Close()
}

var didGET, didPOST, didDELETE bool

func initKeyHandlers(t *testing.T, keyTitle string) {
	didGET = false
	didPOST = false
	didDELETE = false

	mux.HandleFunc("/repos/o/r/keys", func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL)
		if r.Method == "GET" {
			fmt.Fprint(w, `[{"id":1,"title":"`+keyTitle+`"}]`)
			didGET = true
		} else if r.Method == "POST" {
			didPOST = true
		}
	})

	mux.HandleFunc("/repos/o/r/keys/1", func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL)
		testMethod(t, r, "DELETE")
		didDELETE = true
	})
}

func TestInsertDeployKey_KeyDoesntExist(t *testing.T) {
	setup()
	defer teardown()
	initKeyHandlers(t, "doesntMatch")

	g := Github{
		client: client,
	}

	err := g.InsertDeployKey("o", "r", "ssh-rsa AAA", "test-deploy-key")
	if err != nil {
		t.Fatal(err)
	}
	if didGET != true {
		t.Fatal("Should have requested keys")
	}
	if didPOST != true {
		t.Fatal("Should have created key")
	}
	if didDELETE != false {
		t.Fatal("Should have not deleted key")
	}
}

func TestInsertDeployKey_KeyDoesExist(t *testing.T) {
	setup()
	defer teardown()
	initKeyHandlers(t, "test-deploy-key")

	g := Github{
		client: client,
	}

	err := g.InsertDeployKey("o", "r", "ssh-rsa AAA", "test-deploy-key")
	if err != nil {
		t.Fatal(err)
	}
	if didGET != true {
		t.Fatal("Should have requested keys")
	}
	if didPOST != true {
		t.Fatal("Should have created key")
	}
	if didDELETE != true {
		t.Fatal("Should have deleted key")
	}
}

func initRepoHandlers(t *testing.T) {
	mux.HandleFunc("/user/repos", func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL)
		if r.Method == "GET" {
			if r.URL.Query().Get("page") == "2" {
				fmt.Fprint(w, `
					[{
					"id": 3,
					"owner": {
						"id": 100,
						"login": "weaveworks"
					},
					"name": "service-ui",
					"full_name": "weaveworks/service-ui",
					"description": "Weaveworks Service UI Repo",
					"ssh_url": "git:github.com/weaveworks/service-ui.git"
					}]
				`)
			} else {
				w.Header().Set("Link", "<https://api.github.com/user/repos?page=2>; rel=\"next\"")
				fmt.Fprint(w, `
					[{
					"id": 1,
					"owner": {
						"id": 100,
						"login": "weaveworks"
					},
					"name": "service",
					"full_name": "weaveworks/service",
					"description": "Weaveworks Service Repo",
					"ssh_url": "git:github.com/weaveworks/service.git"
					}, {
					"id": 2,
					"owner": {
						"id": 100,
						"login": "weaveworks"
					},
					"name": "service-conf",
					"full_name": "weaveworks/service-conf",
					"description": "Weaveworks Service Conf Repo",
					"ssh_url": "git:github.com/weaveworks/service-conf.git"
					}]
				`)
			}

		}
	})
}

func TestGetRepos(t *testing.T) {
	setup()
	defer teardown()
	initRepoHandlers(t)

	g := Github{
		client: client,
	}

	repos, err := g.GetRepos()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(repos), 3)
}

func testMethod(t *testing.T, r *http.Request, want string) {
	if got := r.Method; got != want {
		t.Errorf("Request method: %v, want %v", got, want)
	}
}
