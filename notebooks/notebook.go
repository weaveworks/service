package notebooks

import (
	"time"

	"github.com/satori/go.uuid"
)

// Notebook describes a collection of query entries for an instance
type Notebook struct {
	ID        uuid.UUID       `json:"id"`
	OrgID     string          `json:"org_id"`
	AuthorID  string          `json:"author"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Title     string          `json:"title"`
	Entries   []NotebookEntry `json:"entries"`
}

// NotebookEntry describes a query for an instance
type NotebookEntry struct {
	Query      string `json:"query"`
	QueryEnd   string `json:"queryEnd"`
	QueryRange string `json:"queryRange"`
	Type       string `json:"type"`
}
