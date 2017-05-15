package notebooks

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/satori/go.uuid"
	"github.com/weaveworks/service/users"
)

// Errors
var (
	ErrNotebookVersionMismatch = errors.New("notebook version mismatch")
)

// Notebook describes a collection of PromQL queries
type Notebook struct {
	ID             uuid.UUID   `json:"id"`
	OrgID          string      `json:"-"`
	CreatedBy      string      `json:"-"`
	CreatedAt      time.Time   `json:"-"`
	UpdatedBy      string      `json:"-"`
	UpdatedByEmail string      `json:"updatedByEmail"` // resolved with ResolveUser
	UpdatedAt      time.Time   `json:"updatedAt"`
	Title          string      `json:"title"`
	Entries        []Entry     `json:"entries"`
	QueryEnd       json.Number `json:"queryEnd"`
	QueryRange     string      `json:"queryRange"`
	TrailingNow    bool        `json:"trailingNow"`
	Version        uuid.UUID   `json:"version"`
}

// ResolveUser uses the UserClient to fill in details about the user such as email address
func (n *Notebook) ResolveUser(r *http.Request, usersClient users.UsersClient) error {
	userResponse, err := usersClient.GetUser(r.Context(), &users.GetUserRequest{UserID: n.UpdatedBy})
	if err != nil {
		return err
	}
	n.UpdatedByEmail = userResponse.User.Email
	return nil
}

// Entry describes a PromQL query for a notebook
type Entry struct {
	Query      string      `json:"query"`
	QueryEnd   json.Number `json:"queryEnd"`
	QueryRange string      `json:"queryRange"`
	Type       string      `json:"type"`
}
