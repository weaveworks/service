package api_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/service/prom"
	"github.com/weaveworks/service/prom/api"
)

func TestAPI_getAllNotebooks(t *testing.T) {
	setup(t)
	defer cleanup(t)

	notebookEntry := prom.NotebookEntry{Query: "metric{}", QueryEnd: 1000.1, QueryRange: "1h", Type: "graph"}
	notebooks := []prom.Notebook{
		{
			OrgID:     "org1",
			AuthorID:  "user1",
			UpdatedAt: time.Now(),
			Title:     "Test notebook 1",
			Entries:   []prom.NotebookEntry{notebookEntry},
		},
		{
			OrgID:     "org1",
			AuthorID:  "user2",
			UpdatedAt: time.Now(),
			Title:     "Test notebook 2",
			Entries:   []prom.NotebookEntry{notebookEntry},
		},
		{
			OrgID:     "org2",
			AuthorID:  "user1",
			UpdatedAt: time.Now(),
			Title:     "Other org notebook",
			Entries:   []prom.NotebookEntry{notebookEntry},
		},
	}
	for _, notebook := range notebooks {
		database.CreateNotebook(notebook)
	}

	var result []prom.Notebook
	w := requestAsUser(t, "org1", "user1", "GET", "/api/prom/notebooks", nil)
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.Len(t, result, 2)

	assert.Equal(t, result[0].OrgID, "org1")
	assert.Equal(t, result[0].AuthorID, "user1")
	assert.Equal(t, result[0].Title, "Test notebook 1")
	assert.Equal(t, result[0].Entries, []prom.NotebookEntry{notebookEntry})
	assert.NotEmpty(t, result[0].UpdatedAt)

	assert.Equal(t, result[1].OrgID, "org1")
	assert.Equal(t, result[1].AuthorID, "user2")
	assert.Equal(t, result[1].Title, "Test notebook 2")
	assert.Equal(t, result[1].Entries, []prom.NotebookEntry{notebookEntry})
	assert.NotEmpty(t, result[1].UpdatedAt)
}

func TestAPI_createNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	notebookEntry := prom.NotebookEntry{Query: "metric{}", QueryEnd: 1000.1, QueryRange: "1h", Type: "graph"}
	data := api.CreateNotebook{
		Title:   "New notebook",
		Entries: []prom.NotebookEntry{notebookEntry},
	}

	b, err := json.Marshal(data)
	require.NoError(t, err)

	var result prom.Notebook
	w := requestAsUser(t, "org1", "user1", "POST", "/api/prom/notebooks", bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.UpdatedAt)
	assert.Equal(t, result.OrgID, "org1")
	assert.Equal(t, result.AuthorID, "user1")
	assert.Equal(t, result.Title, "New notebook")
	assert.Equal(t, result.Entries, []prom.NotebookEntry{notebookEntry})

	var results []prom.Notebook
	w = requestAsUser(t, "org1", "user1", "GET", "/api/prom/notebooks", nil)
	err = json.Unmarshal(w.Body.Bytes(), &results)
	assert.NoError(t, err, "Could not unmarshal JSON")
	assert.Len(t, results, 1)
	assert.Equal(t, result.ID, results[0].ID)
}
