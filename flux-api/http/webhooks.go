package http

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/weaveworks/flux/api/v9"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/service/common/constants/webhooks"
)

func (s Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	integrationType := r.Header.Get(webhooks.WebhooksIntegrationTypeHeader)

	switch integrationType {
	case webhooks.GithubPushIntegrationType:
		handleGithubHook(s, w, r)
		return
	default:
		transport.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("Invalid integration type"))
		return
	}
}

func handleGithubHook(s Server, w http.ResponseWriter, r *http.Request) {
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
		// Ignore the error returned here as the sender doesn't care. We'll log any
		// errors at the daemon level.
		s.daemonProxy.NotifyChange(ctx, change)
	default:
		log.Printf("received webhook: %T\n%s", hook, github.Stringify(hook))
	}

	w.WriteHeader(http.StatusOK)
}
