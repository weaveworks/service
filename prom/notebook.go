package main

// Notebook describes a collection of query entries for an instance
type Notebook struct {
	ID          string          `json:"id"`
	Author      string          `json:"author"`
	UpdatedAt   int             `json:"updatedAt"`
	Entries     []NotebookEntry `json:"entries"`
	Title       string          `json:"title"`
	TrailingNow bool            `json:"trailingNow"`
}

// NotebookEntry describes a query for an instance
type NotebookEntry struct {
	ID         string  `json:"id"`
	Query      string  `json:"query"`
	QueryEnd   float32 `json:"queryEnd"`
	QueryRange string  `json:"queryRange"`
	Type       string  `json:"type"`
}
