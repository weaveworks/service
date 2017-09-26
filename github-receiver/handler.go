package main

import (
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/flux/api/v9"
	"github.com/weaveworks/service/common"
	fluxhttp "github.com/weaveworks/service/flux-api/http"
)

var router *mux.Router

func init() {
	router = fluxhttp.NewServiceRouter()
	// Test creating the route here so we can safely ignore errors later.
	_, err := router.GetRoute("GitPushNotify").URL("instance", "")
	if err != nil {
		panic(err)
	}
}

type handler struct {
	fluxURL string
	secret  []byte
}

func makeHandler(u string, s []byte) *handler {
	return &handler{
		fluxURL: u,
		secret:  s,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, h.secret)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	hook, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	}
	w.WriteHeader(http.StatusOK)

	switch hook := hook.(type) {
	case *github.PushEvent:
		instID := r.FormValue("instance")

		update := v9.GitUpdate{
			URL:    *hook.Repo.SSHURL,
			Branch: strings.TrimPrefix(*hook.Ref, "refs/heads/"),
		}

		client := common.NewJSONClient(http.DefaultClient)

		err := client.Post(r.Context(), "", h.makeNotifyURL(instID), update, nil)
		if err != nil {
			log.Error(err)
		}
	default:
		log.Printf("received webhook: %T\n%s", hook, github.Stringify(hook))
	}
}

func (h *handler) makeNotifyURL(instID string) string {
	url, _ := router.GetRoute("GitPushNotify").URL("instance", instID)
	url.Scheme = "http"
	url.Host = h.fluxURL
	return url.String()
}
