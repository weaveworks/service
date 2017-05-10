package notebooks

import (
	"encoding/json"
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
	OrgID     string    `json:"orgId"`
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
	ID         string      `json:"id"`
	Query      string      `json:"query"`
	QueryEnd   json.Number `json:"queryEnd"`
	QueryRange string      `json:"queryRange"`
	Type       string      `json:"type"`
}
