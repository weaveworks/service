package notebooks

import (
	"errors"
	"time"

	"github.com/satori/go.uuid"
)

// Errors
var (
	ErrNotebookVersionMismatch = errors.New("notebook version mismatch")
)

// Notebook describes a collection of PromQL queries
type Notebook struct {
	ID        uuid.UUID `json:"id"`
	OrgID     string    `json:"org_id"`
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedBy string    `json:"updatedBy"`
	UpdatedAt time.Time `json:"updatedAt"`
	Title     string    `json:"title"`
	Entries   []Entry   `json:"entries"`
	Version   uuid.UUID `json:"version"`
}

// Entry describes a PromQL query for a notebook
type Entry struct {
	Query      string `json:"query"`
	QueryEnd   string `json:"queryEnd"`
	QueryRange string `json:"queryRange"`
	Type       string `json:"type"`
}
