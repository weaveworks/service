package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/weaveworks/flux/api/v9"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/service/common/constants/webhooks"
)

const fluxDaemonTimeout = 5 * time.Second

func (s Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	integrationType := r.Header.Get(webhooks.WebhooksIntegrationTypeHeader)

	switch integrationType {
	case webhooks.GithubPushIntegrationType:
		handleGithub(s, w, r)
		return
	case webhooks.DockerHubIntegrationType:
		handleDockerHub(s, w, r)
		return
	case webhooks.QuayIntegrationType:
		handleQuay(s, w, r)
		return
	default:
		transport.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("Invalid integration type"))
		return
	}
}

func handleGithub(s Server, w http.ResponseWriter, r *http.Request) {
	var payload []byte
	switch contentType := r.Header.Get("Content-Type"); contentType {
	case "application/x-www-form-urlencoded":
		if err := r.ParseForm(); err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
		payload = []byte(r.Form.Get("payload"))
	case "application/json":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
		payload = body
	default:
		transport.ErrorResponse(w, r, fmt.Errorf("Unknown content type %q for webhook", contentType))
		return
	}
	hook, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		transport.ErrorResponse(w, r, err)
		return
	}

	switch hook := hook.(type) {
	case *github.PushEvent:
		update := v9.GitUpdate{
			URL:    *hook.Repo.SSHURL,
			Branch: strings.TrimPrefix(*hook.Ref, "refs/heads/"),
		}
		change := v9.Change{
			Kind:   v9.GitChange,
			Source: update,
		}
		ctx := getRequestContext(r)
		ctx, cancel := context.WithTimeout(ctx, fluxDaemonTimeout)
		defer cancel()

		err := s.daemonProxy.NotifyChange(ctx, change)
		if err != nil {
			select {
			case <-ctx.Done():
				fmt.Fprintf(w, "No response from your Weave Cloud agent.")
				w.WriteHeader(http.StatusRequestTimeout)
			default:
				transport.ErrorResponse(w, r, err)
			}
			return
		}
	default:
		log.Printf("received webhook: %T\n%s", hook, github.Stringify(hook))
	}

	w.WriteHeader(http.StatusOK)
}

func handleDockerHub(s Server, w http.ResponseWriter, r *http.Request) {
	// From https://docs.docker.com/docker-hub/webhooks/
	type payload struct {
		Repository struct {
			RepoName string `json:"repo_name"`
		} `json:"repository"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	handleImageNotify(s, w, r, p.Repository.RepoName)
}

func handleQuay(s Server, w http.ResponseWriter, r *http.Request) {
	type payload struct {
		DockerURL string `json:"docker_url"`
	}
	var p payload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	handleImageNotify(s, w, r, p.DockerURL)
}

func handleImageNotify(s Server, w http.ResponseWriter, r *http.Request, img string) {
	ref, err := image.ParseRef(img)
	if err != nil {
		transport.WriteError(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	change := v9.Change{
		Kind: v9.ImageChange,
		Source: v9.ImageUpdate{
			Name: ref.Name,
		},
	}
	ctx := getRequestContext(r)
	// Ignore the error returned here as the sender doesn't care. We'll log any
	// errors at the daemon level.
	s.daemonProxy.NotifyChange(ctx, change)

	w.WriteHeader(http.StatusOK)
}
