package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/service/notebooks"
	"github.com/weaveworks/service/notebooks/api"
)

func TestAPI_listNotebooks(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebooks
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	ns := []notebooks.Notebook{
		{
			OrgID:     "org1",
			CreatedBy: "user1",
			UpdatedBy: "user1",
			Title:     "Test notebook 1",
			Entries:   []notebooks.Entry{notebookEntry},
		},
		{
			OrgID:     "org1",
			CreatedBy: "user2",
			UpdatedBy: "user2",
			Title:     "Test notebook 2",
			Entries:   []notebooks.Entry{notebookEntry},
		},
		{
			OrgID:     "org2",
			CreatedBy: "user1",
			UpdatedBy: "user1",
			Title:     "Other org notebook",
			Entries:   []notebooks.Entry{notebookEntry},
		},
	}
	for _, notebook := range ns {
		b, err := json.Marshal(notebook)
		require.NoError(t, err)
		requestAsUser(t, notebook.OrgID, notebook.UpdatedBy, "POST", "/api/prom/notebooks", bytes.NewReader(b))
	}

	// List all notebooks and check result
	var result api.NotebooksView
	w := requestAsUser(t, "org1", "user1", "GET", "/api/prom/notebooks", nil)
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.Len(t, result.Notebooks, 2)

	// Assert notebooks in descending UpdatedAt (creation time)
	assert.Equal(t, result.Notebooks[0].OrgID, "org1")
	assert.Equal(t, result.Notebooks[0].CreatedBy, "user2")
	assert.Equal(t, result.Notebooks[0].Title, "Test notebook 2")
	assert.Equal(t, result.Notebooks[0].Entries, []notebooks.Entry{notebookEntry})
	assert.NotEmpty(t, result.Notebooks[0].UpdatedAt)

	assert.Equal(t, result.Notebooks[1].OrgID, "org1")
	assert.Equal(t, result.Notebooks[1].CreatedBy, "user1")
	assert.Equal(t, result.Notebooks[1].Title, "Test notebook 1")
	assert.Equal(t, result.Notebooks[1].Entries, []notebooks.Entry{notebookEntry})
	assert.NotEmpty(t, result.Notebooks[1].UpdatedAt)
}

func TestAPI_createNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	data := api.NotebookWriteView{
		Title:   "New notebook",
		Entries: []notebooks.Entry{notebookEntry},
	}

	b, err := json.Marshal(data)
	require.NoError(t, err)

	// Make request to create notebook
	var result notebooks.Notebook
	w := requestAsUser(t, "org1", "user1", "POST", "/api/prom/notebooks", bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.UpdatedAt)
	assert.Equal(t, result.OrgID, "org1")
	assert.Equal(t, result.CreatedBy, "user1")
	assert.Equal(t, result.UpdatedBy, "user1")
	assert.Equal(t, result.Title, "New notebook")
	assert.Equal(t, result.Entries, []notebooks.Entry{notebookEntry})

	// Check it was created
	var getResult notebooks.Notebook
	w = requestAsUser(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", result.ID.String()), nil)
	err = json.Unmarshal(w.Body.Bytes(), &getResult)
	assert.NoError(t, err, "Could not unmarshal JSON")
	assert.Equal(t, result, getResult)
}

func TestAPI_getNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebook
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := notebooks.Notebook{
		OrgID:     "org1",
		CreatedBy: "user1",
		UpdatedBy: "user1",
		Title:     "Test notebook",
		Entries:   []notebooks.Entry{notebookEntry},
	}

	var createResult notebooks.Notebook
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	w := requestAsUser(t, notebook.OrgID, notebook.UpdatedBy, "POST", "/api/prom/notebooks", bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &createResult)
	assert.NoError(t, err, "Could not unmarshal JSON")

	// Get notebook
	var result notebooks.Notebook
	w = requestAsUser(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	// Check individual fields as some are updated by the database
	assert.Equal(t, result.ID, createResult.ID)
	assert.Equal(t, result.OrgID, "org1")
	assert.Equal(t, result.CreatedBy, "user1")
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

	// Create notebook
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := notebooks.Notebook{
		OrgID:     "org1",
		CreatedBy: "user1",
		UpdatedBy: "user1",
		Title:     "Test notebook",
		Entries:   []notebooks.Entry{notebookEntry},
	}

	var createResult notebooks.Notebook
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	w := requestAsUser(t, notebook.OrgID, notebook.UpdatedBy, "POST", "/api/prom/notebooks", bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &createResult)
	assert.NoError(t, err, "Could not unmarshal JSON")

	// Create update request
	updatedNotebookEntry := notebooks.Entry{Query: "updatedMetric{}", QueryEnd: "77.7", QueryRange: "7h", Type: "new"}
	data := api.NotebookWriteView{
		Title:   "Updated notebook",
		Entries: []notebooks.Entry{updatedNotebookEntry},
	}
	b, err = json.Marshal(data)
	require.NoError(t, err)

	// Make request to update notebook with ID notebookID2
	var result notebooks.Notebook
	w = requestAsUser(t, "org1", "user1", "PUT", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	// Check individual fields as updated times have changed
	assert.Equal(t, result.ID, createResult.ID)
	assert.Equal(t, result.Title, "Updated notebook")
	assert.True(t, result.UpdatedAt.After(createResult.UpdatedAt))

	// Check the update is persistent
	var getResult notebooks.Notebook
	w = requestAsUser(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	err = json.Unmarshal(w.Body.Bytes(), &getResult)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.Equal(t, getResult.Title, "Updated notebook")
	assert.Equal(t, getResult.Entries[0].Query, "updatedMetric{}")
	assert.Equal(t, getResult.Entries[0].QueryEnd, "77.7")
	assert.Equal(t, getResult.Entries[0].QueryRange, "7h")
	assert.Equal(t, getResult.Entries[0].Type, "new")
}

func TestAPI_deleteNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebook in database
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := notebooks.Notebook{
		OrgID:     "org1",
		CreatedBy: "user1",
		UpdatedBy: "user1",
		Title:     "Test notebook",
		Entries:   []notebooks.Entry{notebookEntry},
	}

	var createResult notebooks.Notebook
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	w := requestAsUser(t, notebook.OrgID, notebook.UpdatedBy, "POST", "/api/prom/notebooks", bytes.NewReader(b))
	err = json.Unmarshal(w.Body.Bytes(), &createResult)
	assert.NoError(t, err, "Could not unmarshal JSON")

	// Make request to update notebook with ID notebookID2
	w = requestAsUser(t, "org1", "user1", "DELETE", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	assert.Equal(t, w.Code, http.StatusOK)

	// Check it was deleted
	w = requestAsUser(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	assert.Equal(t, w.Code, 500)
}
