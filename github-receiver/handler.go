package main

import (
	"net/http"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
)

type handler struct {
	fluxSvcURL string
	secret     []byte
}

func makeHandler(u string, s []byte) *handler {
	return &handler{
		fluxSvcURL: u,
		secret:     s,
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

	switch hook := hook.(type) {
	case *github.PushEvent:
		log.Println("received push event:", *hook.Repo.SSHURL, *hook.Ref)
	default:
		log.Printf("received webhook: %T\n%s", hook, github.Stringify(hook))
	}

}
