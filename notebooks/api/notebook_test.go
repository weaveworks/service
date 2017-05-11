package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/service/notebooks"
	"github.com/weaveworks/service/notebooks/api"
)

func makeNotebookRequest(t *testing.T, orgID, userID, method, url string, data []byte) (*httptest.ResponseRecorder, notebooks.Notebook) {
	var result notebooks.Notebook
	w := requestAsUser(t, orgID, userID, method, url, bytes.NewReader(data))
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")
	return w, result
}

func TestAPI_listNotebooks(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebooks
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}

	ns := []struct {
		orgID    string
		userID   string
		notebook api.NotebookWriteView
	}{
		{
			orgID:  "org1",
			userID: "user1",
			notebook: api.NotebookWriteView{
				Title:   "Test notebook 1",
				Entries: []notebooks.Entry{notebookEntry},
			},
		},
		{
			orgID:  "org1",
			userID: "user2",

			notebook: api.NotebookWriteView{
				Title:   "Test notebook 2",
				Entries: []notebooks.Entry{notebookEntry},
			},
		},
		{
			orgID:  "org2",
			userID: "user1",
			notebook: api.NotebookWriteView{
				Title:   "Other org notebook",
				Entries: []notebooks.Entry{notebookEntry},
			},
		},
	}
	for _, n := range ns {
		b, err := json.Marshal(n.notebook)
		require.NoError(t, err)
		requestAsUser(t, n.orgID, n.userID, "POST", "/api/prom/notebooks", bytes.NewReader(b))
	}

	// List all notebooks and check result
	var result api.NotebooksView
	w := requestAsUser(t, "org1", "user1", "GET", "/api/prom/notebooks", nil)
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err, "Could not unmarshal JSON")

	assert.Len(t, result.Notebooks, 2)

	// Assert notebooks in descending UpdatedAt (creation time)
	assert.Equal(t, result.Notebooks[0].OrgID, "org1")
	assert.Equal(t, result.Notebooks[0].CreatedBy, "user1")
	assert.Equal(t, result.Notebooks[0].Title, "Test notebook 1")
	assert.Equal(t, result.Notebooks[0].Entries, []notebooks.Entry{notebookEntry})
	assert.NotEmpty(t, result.Notebooks[0].UpdatedAt)

	assert.Equal(t, result.Notebooks[1].OrgID, "org1")
	assert.Equal(t, result.Notebooks[1].CreatedBy, "user2")
	assert.Equal(t, result.Notebooks[1].Title, "Test notebook 2")
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
	w, result := makeNotebookRequest(t, "org1", "user1", "POST", "/api/prom/notebooks", b)

	assert.Equal(t, w.Code, 200)
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.UpdatedAt)
	assert.NotEmpty(t, result.Version)
	assert.Equal(t, result.OrgID, "org1")
	assert.Equal(t, result.CreatedBy, "user1")
	assert.Equal(t, result.UpdatedBy, "user1")
	assert.Equal(t, result.Title, "New notebook")
	assert.Equal(t, result.Entries, []notebooks.Entry{notebookEntry})

	// Check it was created
	w, getResult := makeNotebookRequest(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", result.ID.String()), nil)
	assert.Equal(t, w.Code, 200)
	assert.Equal(t, result, getResult)
}

func TestAPI_getNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebook
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := api.NotebookWriteView{
		Title:   "Test notebook",
		Entries: []notebooks.Entry{notebookEntry},
	}
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	_, createResult := makeNotebookRequest(t, "org1", "user1", "POST", "/api/prom/notebooks", b)

	w, result := makeNotebookRequest(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	assert.Equal(t, w.Code, 200)

	// Check individual fields as some are updated by the database
	assert.Equal(t, result.ID, createResult.ID)
	assert.Equal(t, result.OrgID, "org1")
	assert.Equal(t, result.CreatedBy, "user1")
	assert.NotEmpty(t, result.UpdatedAt)
	assert.Equal(t, result.Title, "Test notebook")

	assert.Len(t, result.Entries, 1)
	assert.Equal(t, result.Entries[0].Query, "metric{}")
	assert.Equal(t, result.Entries[0].QueryEnd.String(), "1000.1")
	assert.Equal(t, result.Entries[0].QueryRange, "1h")
	assert.Equal(t, result.Entries[0].Type, "graph")
}

func TestAPI_getNotebook_doesNotExist(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Get non-existent notebook
	w := requestAsUser(t, "org1", "user1", "GET", "/api/prom/notebooks/1", nil)
	assert.Equal(t, w.Code, 404)
}

func TestAPI_updateNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebook
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := api.NotebookWriteView{
		Title:   "Test notebook",
		Entries: []notebooks.Entry{notebookEntry},
	}
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	_, createResult := makeNotebookRequest(t, "org1", "user1", "POST", "/api/prom/notebooks", b)
	initialVersion := createResult.Version

	// Create update request
	updatedNotebookEntry := notebooks.Entry{Query: "updatedMetric{}", QueryEnd: "77.7", QueryRange: "7h", Type: "new"}
	data := api.NotebookWriteView{
		Title:   "Updated notebook",
		Entries: []notebooks.Entry{updatedNotebookEntry},
	}
	b, err = json.Marshal(data)
	require.NoError(t, err)

	w, result := makeNotebookRequest(t, "org1", "user1", "PUT", fmt.Sprintf("/api/prom/notebooks/%s?version=%s", createResult.ID, createResult.Version), b)
	assert.Equal(t, w.Code, 200)

	// Check individual fields as some fields have changed
	// TODO: UpdatedTime not tested because of transaction
	assert.Equal(t, result.ID, createResult.ID)
	assert.Equal(t, result.Title, "Updated notebook")
	assert.NotEqual(t, result.Version, initialVersion)

	// Check the update is persistent
	w, getResult := makeNotebookRequest(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)

	assert.Equal(t, w.Code, 200)
	assert.Equal(t, getResult.Title, "Updated notebook")
	assert.Equal(t, getResult.Version, result.Version)
	assert.Equal(t, getResult.Entries[0].Query, "updatedMetric{}")
	assert.Equal(t, getResult.Entries[0].QueryEnd.String(), "77.7")
	assert.Equal(t, getResult.Entries[0].QueryRange, "7h")
	assert.Equal(t, getResult.Entries[0].Type, "new")
}

func TestAPI_updateNotebook_wrongVersion(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebook
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := api.NotebookWriteView{
		Title:   "Test notebook",
		Entries: []notebooks.Entry{notebookEntry},
	}
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	w, createResult := makeNotebookRequest(t, "org1", "user1", "POST", "/api/prom/notebooks", b)
	initialVersion := createResult.Version

	// Create update request
	updatedNotebookEntry := notebooks.Entry{Query: "updatedMetric{}", QueryEnd: "77.7", QueryRange: "7h", Type: "new"}
	data := api.NotebookWriteView{
		Title:   "Updated notebook",
		Entries: []notebooks.Entry{updatedNotebookEntry},
	}
	b, err = json.Marshal(data)
	require.NoError(t, err)

	// Make request to update notebook with wrong version
	w = requestAsUser(t, "org1", "user1", "PUT", fmt.Sprintf("/api/prom/notebooks/%s?version=%s", createResult.ID, "invalid-version"), bytes.NewReader(b))
	assert.Equal(t, w.Code, 409)

	// Check the update did not happen
	w, getResult := makeNotebookRequest(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	assert.Equal(t, w.Code, 200)
	assert.Equal(t, getResult.Title, "Test notebook")
	assert.Equal(t, getResult.Version, initialVersion)
}

func TestAPI_deleteNotebook(t *testing.T) {
	setup(t)
	defer cleanup(t)

	// Create notebook in database
	notebookEntry := notebooks.Entry{Query: "metric{}", QueryEnd: "1000.1", QueryRange: "1h", Type: "graph"}
	notebook := api.NotebookWriteView{
		Title:   "Test notebook",
		Entries: []notebooks.Entry{notebookEntry},
	}
	b, err := json.Marshal(notebook)
	require.NoError(t, err)
	w, createResult := makeNotebookRequest(t, "org1", "user1", "POST", "/api/prom/notebooks", b)

	// Make request to update notebook with ID notebookID2
	w = requestAsUser(t, "org1", "user1", "DELETE", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	assert.Equal(t, w.Code, http.StatusNoContent)

	// Check it was deleted
	w = requestAsUser(t, "org1", "user1", "GET", fmt.Sprintf("/api/prom/notebooks/%s", createResult.ID), nil)
	assert.Equal(t, w.Code, http.StatusNotFound)
}
