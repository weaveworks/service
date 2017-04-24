package prom

// Notebook describes a collection of query entries for an instance
type Notebook struct {
	ID        string          `json:"id"`
	OrgID     string          `json:"org_id"`
	AuthorID  string          `json:"author"`
	UpdatedAt int             `json:"updatedAt"`
	Entries   []NotebookEntry `json:"entries"`
	Title     string          `json:"title"`
}

// NotebookEntry describes a query for an instance
type NotebookEntry struct {
	ID         string  `json:"id"`
	Query      string  `json:"query"`
	QueryEnd   float32 `json:"queryEnd"`
	QueryRange string  `json:"queryRange"`
	Type       string  `json:"type"`
}
