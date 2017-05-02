package api

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/prom"

	"github.com/satori/go.uuid"
)

// listNotebooks returns all of the notebooks for an instance
func (a *API) listNotebooks(w http.ResponseWriter, r *http.Request) {
	orgID, _, err := user.ExtractFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebooks, err := a.db.ListNotebooks(orgID)
	if err != nil {
		log.Errorf("Error getting notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebooks); err != nil {
		log.Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// NotebookWriteView describes the structure the user can write to
type NotebookWriteView struct {
	Title   string               `json:"title"`
	Entries []prom.NotebookEntry `json:"entries"`
}

// createNotebook creates a notebook
func (a *API) createNotebook(w http.ResponseWriter, r *http.Request) {
	orgID, _, err := user.ExtractFromHTTPRequest(r)
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

	notebook := prom.Notebook{
		ID:        uuid.NewV4(),
		OrgID:     orgID,
		AuthorID:  r.Header.Get("X-Scope-UserID"),
		UpdatedAt: time.Now(),
		Title:     input.Title,
		Entries:   input.Entries,
	}

	err = a.db.CreateNotebook(notebook)
	if err != nil {
		log.Errorf("Error creating notebook: %v", err)
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

	orgID, _, err := user.ExtractFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebook, err := a.db.GetNotebook(notebookID, orgID)
	if err != nil {
		log.Errorf("Error getting notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	orgID, _, err := user.ExtractFromHTTPRequest(r)
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

	notebook := prom.Notebook{
		AuthorID:  r.Header.Get("X-Scope-UserID"),
		UpdatedAt: time.Now(),
		Title:     input.Title,
		Entries:   input.Entries,
	}

	err = a.db.UpdateNotebook(notebookID, orgID, notebook)
	if err != nil {
		log.Errorf("Error updating notebook: %v", err)
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
