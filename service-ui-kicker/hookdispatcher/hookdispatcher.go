package hookdispatcher

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"
)

type listenerMap map[string]chan interface{}

// HookDispatcher handles webhooks and dispatches them to listeners
type HookDispatcher struct {
	mu        sync.Mutex
	listeners listenerMap
}

// New returns new hook server
func New() *HookDispatcher {
	return &HookDispatcher{listeners: make(listenerMap)}
}

// Listen registers a listener for a repo
func (hs *HookDispatcher) Listen(repo string) chan interface{} {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	ch := make(chan interface{}, 8)
	hs.listeners[repo] = ch
	return ch
}

// HandlePush handles GitHub push events
func (hs *HookDispatcher) HandlePush(payload interface{}, header webhooks.Header) {
	pl := payload.(github.PushPayload)
	url := pl.Repository.CloneURL
	hs.mu.Lock()
	ch, ok := hs.listeners[url]
	hs.mu.Unlock()
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
	hs.mu.Lock()
	ch, ok := hs.listeners[url]
	hs.mu.Unlock()
	if ok {
		ch <- payload
	} else {
		log.WithField("repository", url).Errorf("Discarding status payload from unhandled repository")
	}
}
