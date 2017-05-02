package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/service/prom"
	"github.com/weaveworks/service/prom/api"

	"github.com/satori/go.uuid"
)

func TestAPI_listNotebooks(t *testing.T) {
	setup(t)
	defer cleanup(t)

	notebookEntry := prom.NotebookEntry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
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

	notebookEntry := prom.NotebookEntry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	data := api.NotebookWriteView{
		Title:   "New notebook",
		Entries: []prom.NotebookEntry{notebookEntry},
	}

	b, err := json.Marshal(data)
	require.NoError(t, err)

	// Make request to create notebook
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

	// Check it was created in the DB
	notebook, err := database.GetNotebook(result.ID.String(), result.OrgID)
	assert.NoError(t, err, "Could not fetch notebook from DB")
	assert.Equal(t, notebook.Title, "New notebook")
}

func TestAPI_getNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	notebookID := uuid.NewV4()
	notebookEntry := prom.NotebookEntry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := prom.Notebook{
		ID:        notebookID,
		OrgID:     "org1",
		AuthorID:  "user1",
		UpdatedAt: time.Now(),
		Title:     "Test notebook",
		Entries:   []prom.NotebookEntry{notebookEntry},
	}
	database.CreateNotebook(notebook)

	var result prom.Notebook
	w := requestAsUser(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", notebookID), nil)
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.Equal(t, result.ID, notebookID)
	assert.Equal(t, result.OrgID, "org1")
	assert.Equal(t, result.AuthorID, "user1")
	assert.NotEmpty(t, result.UpdatedAt)
	assert.Equal(t, result.Title, "Test notebook")

	assert.Len(t, result.Entries, 1)
	assert.Equal(t, result.Entries[0].Query, "metric{}")
	assert.Equal(t, result.Entries[0].QueryEnd, "1000.1")
	assert.Equal(t, result.Entries[0].QueryRange, "1h")
	assert.Equal(t, result.Entries[0].Type, "graph")
}

func TestAPI_updateNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	notebookID1 := uuid.NewV4()
	notebookID2 := uuid.NewV4()
	notebookID3 := uuid.NewV4()

	notebookEntry := prom.NotebookEntry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebooks := []prom.Notebook{
		{
			ID:        notebookID1,
			OrgID:     "org1",
			AuthorID:  "user1",
			UpdatedAt: time.Now(),
			Title:     "Test notebook 1",
			Entries:   []prom.NotebookEntry{notebookEntry},
		},
		{
			ID:        notebookID2,
			OrgID:     "org1",
			AuthorID:  "user2",
			UpdatedAt: time.Now(),
			Title:     "Test notebook 2",
			Entries:   []prom.NotebookEntry{notebookEntry},
		},
		{
			ID:        notebookID3,
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

	updatedNotebookEntry := prom.NotebookEntry{Query: "updatedMetric{}", QueryEnd: "77.7", QueryRange: "7h", Type: "new"}
	data := api.NotebookWriteView{
		Title:   "Updated notebook",
		Entries: []prom.NotebookEntry{updatedNotebookEntry},
	}
	b, err := json.Marshal(data)
	require.NoError(t, err)

	// Make request to update notebook with ID notebookID2
	var result prom.Notebook
	w := requestAsUser(t, "org1", "user1", "PUT", fmt.Sprintf("/api/prom/notebooks/%s", notebookID2), bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	// Check it was updated in the DB
	notebook, err := database.GetNotebook(notebookID2.String(), "org1")
	assert.NoError(t, err, "Could not fetch notebook from DB")
	assert.Equal(t, notebook.Title, "Updated notebook")
	assert.Equal(t, notebook.Entries[0].Query, "updatedMetric{}")
	assert.Equal(t, notebook.Entries[0].QueryEnd, "77.7")
	assert.Equal(t, notebook.Entries[0].QueryRange, "7h")
	assert.Equal(t, notebook.Entries[0].Type, "new")
}
