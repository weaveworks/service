package http

import (
	"net"
	"net/http"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
)

// HostFromRequest extracts the host name portion from the request's remote address
func HostFromRequest(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		user.LogWith(r.Context(), logging.Global()).Errorf("Error splitting '%s': %v", r.RemoteAddr, err)
	}
	return host
}
