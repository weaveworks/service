package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/notebooks"
)

// NotebooksView describes a collection of notebooks
type NotebooksView struct {
	Notebooks []notebooks.Notebook `json:"notebooks"`
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
		logging.With(r.Context()).Errorf("Error getting notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resolvedNotebooks := []notebooks.Notebook{}
	for _, n := range ns {
		err = n.ResolveUser(r, a.usersClient)
		if err != nil {
			logging.With(r.Context()).Errorf("Error resolving notebook user: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resolvedNotebooks = append(resolvedNotebooks, n)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NotebooksView{resolvedNotebooks}); err != nil {
		logging.With(r.Context()).Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// NotebookWriteView describes the structure the user can write to
type NotebookWriteView struct {
	Title       string            `json:"title"`
	Entries     []notebooks.Entry `json:"entries"`
	QueryEnd    json.Number       `json:"queryEnd"`
	QueryRange  string            `json:"queryRange"`
	TrailingNow bool              `json:"trailingNow"`
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
		logging.With(r.Context()).Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebook := notebooks.Notebook{
		OrgID:       orgID,
		CreatedBy:   userID,
		UpdatedBy:   userID,
		Title:       input.Title,
		Entries:     input.Entries,
		QueryEnd:    input.QueryEnd,
		QueryRange:  input.QueryRange,
		TrailingNow: input.TrailingNow,
	}

	id, err := a.db.CreateNotebook(notebook)
	if err != nil {
		logging.With(r.Context()).Errorf("Error creating notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch new notebook to include generated ID and update timestamps
	notebook, err = a.db.GetNotebook(id, orgID)
	if err != nil {
		logging.With(r.Context()).Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = notebook.ResolveUser(r, a.usersClient)
	if err != nil {
		logging.With(r.Context()).Errorf("Error resolving notebook user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		logging.With(r.Context()).Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// getNotebook gets a single notebook with the notebook ID
func (a *API) getNotebook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		logging.With(r.Context()).Error("Missing notebookID var")
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

	err = notebook.ResolveUser(r, a.usersClient)
	if err != nil {
		logging.With(r.Context()).Errorf("Error resolving notebook user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		logging.With(r.Context()).Errorf("Error encoding notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// updateNotebook updates a notebook with the same id
func (a *API) updateNotebook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		logging.With(r.Context()).Error("Missing notebookID var")
		http.Error(w, "Missing notebookID", http.StatusBadRequest)
		return
	}
	vals := r.URL.Query()
	version, ok := vals["version"]
	if !ok {
		logging.With(r.Context()).Error("Missing version val")
		http.Error(w, "Missing version query parameter", http.StatusBadRequest)
		return
	}

	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Create the notebook update
	var input NotebookWriteView
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		logging.With(r.Context()).Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebook := notebooks.Notebook{
		UpdatedBy:   userID,
		Title:       input.Title,
		Entries:     input.Entries,
		QueryEnd:    input.QueryEnd,
		QueryRange:  input.QueryRange,
		TrailingNow: input.TrailingNow,
	}

	err = a.db.UpdateNotebook(notebookID, orgID, notebook, version[0])
	if err == notebooks.ErrNotebookVersionMismatch {
		http.Error(w, "Notebook version mismatch", http.StatusConflict)
		return
	} else if err != nil {
		logging.With(r.Context()).Errorf("Error updating notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the updated notebook which includes updated timestamps
	notebook, err = a.db.GetNotebook(notebookID, orgID)
	if err != nil {
		logging.With(r.Context()).Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = notebook.ResolveUser(r, a.usersClient)
	if err != nil {
		logging.With(r.Context()).Errorf("Error resolving notebook user: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		logging.With(r.Context()).Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// deleteNotebook deletes the notebook with the id
func (a *API) deleteNotebook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		logging.With(r.Context()).Error("Missing notebookID var")
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
		logging.With(r.Context()).Errorf("Error deleting notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
