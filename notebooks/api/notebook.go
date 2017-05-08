package api

import (
	"encoding/json"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notebooks"
)

// NotebooksView describes a collection of notebooks
type NotebooksView struct {
	Notebooks []notebooks.Notebook
}

// listNotebooks returns all of the notebooks for an instance
func (a *API) listNotebooks(w http.ResponseWriter, r *http.Request) {
	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	ns, err := a.db.ListNotebooks(orgID)
	if err != nil {
		log.Errorf("Error getting notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NotebooksView{ns}); err != nil {
		log.Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// NotebookWriteView describes the structure the user can write to
type NotebookWriteView struct {
	Title   string            `json:"title"`
	Entries []notebooks.Entry `json:"entries"`
}

// createNotebook creates a notebook
func (a *API) createNotebook(w http.ResponseWriter, r *http.Request) {
	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var input NotebookWriteView
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebook := notebooks.Notebook{
		OrgID:     orgID,
		CreatedBy: userID,
		UpdatedBy: userID,
		Title:     input.Title,
		Entries:   input.Entries,
	}

	id, err := a.db.CreateNotebook(notebook)
	if err != nil {
		log.Errorf("Error creating notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch new notebook to include generated ID and update timestamps
	notebook, err = a.db.GetNotebook(id, orgID)
	if err != nil {
		log.Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		log.Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getNotebook gets a single notebook with the notebook ID
func (a *API) getNotebook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		log.Error("Missing notebookID var")
		http.Error(w, "Missing notebookID", http.StatusBadRequest)
		return
	}

	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebook, err := a.db.GetNotebook(notebookID, orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		log.Errorf("Error encoding notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// updateNotebook updates a notebook with the same id
func (a *API) updateNotebook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		log.Error("Missing notebookID var")
		http.Error(w, "Missing notebookID", http.StatusBadRequest)
		return
	}
	vals := r.URL.Query()
	version, ok := vals["version"]
	if !ok {
		log.Error("Missing version val")
		http.Error(w, "Missing version query parameter", http.StatusBadRequest)
		return
	}

	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Fetch the current notebook to get the version
	currentNotebook, err := a.db.GetNotebook(notebookID, orgID)
	if err != nil {
		log.Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if version[0] != currentNotebook.Version.String() {
		http.Error(w, "Notebook version mismatch", http.StatusConflict)
		return
	}

	// Create the notebook update
	var input NotebookWriteView
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebook := notebooks.Notebook{
		UpdatedBy: userID,
		Title:     input.Title,
		Entries:   input.Entries,
	}

	err = a.db.UpdateNotebook(notebookID, orgID, notebook)
	if err != nil {
		log.Errorf("Error updating notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the updated notebook which includes updated timestamps
	notebook, err = a.db.GetNotebook(notebookID, orgID)
	if err != nil {
		log.Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		log.Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// deleteNotebook deletes the notebook with the id
func (a *API) deleteNotebook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		log.Error("Missing notebookID var")
		http.Error(w, "Missing notebookID", http.StatusBadRequest)
		return
	}

	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	err = a.db.DeleteNotebook(notebookID, orgID)
	if err != nil {
		log.Errorf("Error deleting notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := w.Write([]byte("OK")); err != nil {
		log.Errorf("Error returning response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
