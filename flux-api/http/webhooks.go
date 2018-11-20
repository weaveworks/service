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

type webhookHandler func(Server, http.ResponseWriter, *http.Request)

var handlers = map[string]webhookHandler{
	webhooks.GithubPushIntegrationType:       handleGithubPush,
	webhooks.BitbucketOrgPushIntegrationType: handleBitbucketOrgPush,
	webhooks.GitlabPushIntegrationType:       handleGitlabPush,
	webhooks.DockerHubIntegrationType:        handleDockerHub,
	webhooks.QuayIntegrationType:             handleQuay,
}

func (s Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	integrationType := r.Header.Get(webhooks.WebhooksIntegrationTypeHeader)

	if handle, ok := handlers[integrationType]; ok {
		handle(s, w, r)
		return
	}
	transport.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("Invalid integration type"))
}

func handleGithubPush(s Server, w http.ResponseWriter, r *http.Request) {
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

// Handily (not handily) Bitbucket's cloud and self-hosted
// products have different names for all the events and fields. This is for the events sent by the "Cloud" product (bitbucket.org):
// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-Push
//
// (For completeness, the docs for the self-hosted Bitbucket "Server"
// are at
// https://confluence.atlassian.com/bitbucketserver/event-payload-938025882.html. The
// self-hosted version includes a signature in the header
// "X-Hub-Signature", but this is not present for "Cloud").

func handleBitbucketOrgPush(s Server, w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Event-Key") != "repo:push" {
		transport.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("Unexpected or missing header X-Event-Key"))
		return
	}

	type bitbucketOrgPayload struct {
		Repository bitbucketOrgRepository
		Push       struct {
			Changes []struct {
				New struct {
					Type, Name string
				}
			}
		}
	}

	var payload bitbucketOrgPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
	}

	// The bitbucket.org events potentially contain many ref updates;
	// presumably, it bundles together e.g., the result of a `git
	// push` into one event. We only notify about one things at a time
	// though, so:
	//  - collect all the changes
	//  - send as many as we can before we reach our deadline.
	// That may mean we miss some, but this is best effort.
	//
	// NB a change can be to a branch or a tag; here we'll send both
	// through, since it's in principle possible to sync to a tag.

	repo := payload.Repository.RepoURL()
	ctx, cancel := context.WithTimeout(getRequestContext(r), fluxDaemonTimeout)
	defer cancel()
	for i := range payload.Push.Changes {
		refChange := payload.Push.Changes[i].New
		change := v9.Change{
			Kind: v9.GitChange,
			Source: v9.GitUpdate{
				URL:    repo,
				Branch: refChange.Name,
			},
		}
		if err := s.daemonProxy.NotifyChange(ctx, change); err != nil {
			transport.ErrorResponse(w, r, err)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

// The fields of repository that we care about
type bitbucketOrgRepository struct {
	FullName string `json:"full_name"`
}

func (r bitbucketOrgRepository) RepoURL() string {
	return fmt.Sprintf("git@bitbucket.org:%s.git", r.FullName)
}

func handleGitlabPush(s Server, w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Gitlab-Event") != "Push Hook" {
		transport.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("Unexpected or missing X-Gitlab-Event"))
		return
	}

	type gitlabPayload struct {
		Ref     string
		Project struct {
			SSHURL string `json:"git_ssh_url"`
		}
	}

	var payload gitlabPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		transport.WriteError(w, r, http.StatusBadRequest, err)
		return
	}

	change := v9.Change{
		Kind: v9.GitChange,
		Source: v9.GitUpdate{
			URL:    payload.Project.SSHURL,
			Branch: strings.TrimPrefix(payload.Ref, "refs/heads/"),
		},
	}

	ctx, cancel := context.WithTimeout(getRequestContext(r), fluxDaemonTimeout)
	defer cancel()
	if err := s.daemonProxy.NotifyChange(ctx, change); err != nil {
		transport.ErrorResponse(w, r, err)
		return
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
