package main

import (
	"log"
	"net/http"

	"github.com/google/go-github/github"
)

type handler struct {
	fluxSvcUrl string
	secret     []byte
}

func makeHandler(u string, s []byte) *handler {
	return &handler{
		fluxSvcUrl: u,
		secret:     s,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, h.secret)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	hook, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	}
	log.Printf("received webhook: %T\n%s", hook, github.Stringify(hook))
}
