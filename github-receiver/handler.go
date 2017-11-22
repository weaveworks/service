package main

import (
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/common"
)

const notifyPath = "/v6/integrations/github/push"

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
	w.WriteHeader(http.StatusOK)

	switch hook := hook.(type) {
	case *github.PushEvent:
		instID := r.FormValue("instance")
		client := common.NewJSONClient(http.DefaultClient)

		log.Println("Posting to", h.makeNotifyURL(instID))
		err := client.Post(r.Context(), "", h.makeNotifyURL(instID), nil, nil)
		if err != nil {
			log.Error(err)
		}
	default:
		log.Printf("received webhook: %T\n%s", hook, github.Stringify(hook))
	}
}

func (h *handler) makeNotifyURL(instID string) string {
	return fmt.Sprintf("http://%s%s?instance=%s", h.fluxSvcURL, notifyPath, instID)
}
