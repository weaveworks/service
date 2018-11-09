package notebooks

import (
	"encoding/json"
	"net/http"
	"time"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
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
	CreatedByEmail string      `json:"createdByEmail"` // resolved with ResolveReferences
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedBy      string      `json:"-"`
	UpdatedByEmail string      `json:"updatedByEmail"` // resolved with ResolveReferences
	UpdatedAt      time.Time   `json:"updatedAt"`
	Title          string      `json:"title"`
	Entries        []Entry     `json:"entries"`
	QueryEnd       json.Number `json:"queryEnd"`
	QueryRange     string      `json:"queryRange"`
	TrailingNow    bool        `json:"trailingNow"`
	Version        uuid.UUID   `json:"version"`
}

// ResolveReferences uses the UserClient to fill in details, such as
// email addresses, about the users referenced in a notebook.
func (n *Notebook) ResolveReferences(r *http.Request, usersClient users.UsersClient) error {
	var errs error
	if userResponse, err := usersClient.GetUser(r.Context(), &users.GetUserRequest{UserID: n.CreatedBy}); err != nil {
		errs = multierror.Append(errs, errors.Wrapf(err, "unable to resolve user %s in notebook %s", n.CreatedBy, n.ID))
	} else {
		n.CreatedByEmail = userResponse.User.Email
	}
	if userResponse, err := usersClient.GetUser(r.Context(), &users.GetUserRequest{UserID: n.UpdatedBy}); err != nil {
		errs = multierror.Append(errs, errors.Wrapf(err, "unable to resolve user %s in notebook %s", n.UpdatedBy, n.ID))
	} else {
		n.UpdatedByEmail = userResponse.User.Email
	}
	return errs
}

// Entry describes a PromQL query for a notebook
type Entry struct {
	Query      string      `json:"query"`
	QueryEnd   json.Number `json:"queryEnd"`
	QueryRange string      `json:"queryRange"`
	Type       string      `json:"type"`
}
