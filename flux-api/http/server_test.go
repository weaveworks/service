package http

import (
	"testing"

	"github.com/weaveworks/flux/http"
)

// This test ensures flux-api implements everything needed by fluxctl.
func TestRouterImplementsServer(t *testing.T) {
	router := NewServiceRouter()
	Server{}.MakeHandler(router, nil)
	err := http.ImplementsServer(router)
	if err != nil {
		t.Error(err)
	}
}
