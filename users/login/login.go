package login

import (
	"encoding/json"
	"time"
)

// Login pairs a user with a login provider they've used (can use) to log in.
type Login struct {
	UserID     string
	Provider   string
	ProviderID string
	Session    json.RawMessage // per-user session information for configuring the client
	CreatedAt  time.Time
}

// LoginsByProvider sorts logins by provider, so we can return them in a consistent order.
type LoginsByProvider []*Login

func (a LoginsByProvider) Len() int           { return len(a) }
func (a LoginsByProvider) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a LoginsByProvider) Less(i, j int) bool { return a[i].Provider < a[j].Provider }
