package hookdispatcher

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"
)

// HookDispatcher handles webhooks and dispatches them to listeners
type HookDispatcher struct {
	listeners map[string]chan interface{}
}

// New returns new hook server
func New() *HookDispatcher {
	return &HookDispatcher{listeners: make(map[string]chan interface{})}
}

// Listen registers a listener for a repo
func (hs *HookDispatcher) Listen(repo string) chan interface{} {
	ch := make(chan interface{}, 8)
	hs.listeners[repo] = ch
	return ch
}

// HandlePush handles GitHub push events
func (hs *HookDispatcher) HandlePush(payload interface{}, header webhooks.Header) {
	pl := payload.(github.PushPayload)
	url := pl.Repository.CloneURL
	ch, ok := hs.listeners[url]
	if ok {
		ch <- payload
	} else {
		log.WithField("repository", url).Errorf("Discarding push payload from unhandled repository")
	}
}

// HandleStatus handles GitHub Commit status updated from the API
func (hs *HookDispatcher) HandleStatus(payload interface{}, header webhooks.Header) {
	pl := payload.(github.StatusPayload)
	url := pl.Repository.CloneURL
	ch, ok := hs.listeners[url]
	if ok {
		ch <- payload
	} else {
		log.WithField("repository", url).Errorf("Discarding status payload from unhandled repository")
	}
}
