package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/permission"
	"github.com/weaveworks/service/notebooks"
	"github.com/weaveworks/service/users"
)

// NotebooksView describes a collection of notebooks
type NotebooksView struct {
	Notebooks []notebooks.Notebook `json:"notebooks"`
}

// healthCheck handles a very simple health check
func (a *API) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// listNotebooks returns all of the notebooks for an instance
func (a *API) listNotebooks(w http.ResponseWriter, r *http.Request) {
	logger := user.LogWith(r.Context(), logging.Global())
	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	ns, err := a.db.ListNotebooks(orgID)
	if err != nil {
		logger.Errorf("Error getting notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resolvedNotebooks := []notebooks.Notebook{}
	for _, n := range ns {
		a.resolveNotebookReferences(r, &n)
		resolvedNotebooks = append(resolvedNotebooks, n)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NotebooksView{resolvedNotebooks}); err != nil {
		logger.Errorf("Error encoding notebooks: %v", err)
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

func (a *API) requirePermissionTo(ctx context.Context, permissionID, orgID, userID string) error {
	_, err := a.usersClient.RequireOrgMemberPermissionTo(ctx, &users.RequireOrgMemberPermissionToRequest{
		OrgID:        &users.RequireOrgMemberPermissionToRequest_OrgInternalID{OrgInternalID: orgID},
		UserID:       userID,
		PermissionID: permission.CreateNotebook,
	})
	return err
}

// createNotebook creates a notebook
func (a *API) createNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := user.LogWith(ctx, logging.Global())
	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	var input NotebookWriteView
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		logger.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := a.requirePermissionTo(ctx, permission.CreateNotebook, orgID, userID); err != nil {
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
		logger.Errorf("Error creating notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch new notebook to include generated ID and update timestamps
	notebook, err = a.db.GetNotebook(id, orgID)
	if err != nil {
		logger.Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.sendNotebook(w, r, notebook)
}

// getNotebook gets a single notebook with the notebook ID
func (a *API) getNotebook(w http.ResponseWriter, r *http.Request) {
	logger := user.LogWith(r.Context(), logging.Global())
	vars := mux.Vars(r)
	notebookID, ok := vars["notebookID"]
	if !ok {
		logger.Errorln("Missing notebookID var")
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

	a.sendNotebook(w, r, notebook)
}

// updateNotebook updates a notebook with the same id
func (a *API) updateNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := user.LogWith(ctx, logging.Global())

	notebookID, ok := mux.Vars(r)["notebookID"]
	if !ok {
		logger.Errorln("Missing notebookID var")
		http.Error(w, "Missing notebookID", http.StatusBadRequest)
		return
	}
	vals := r.URL.Query()
	version, ok := vals["version"]
	if !ok {
		logger.Errorln("Missing version val")
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
		logger.Errorf("Error decoding json body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := a.requirePermissionTo(ctx, permission.UpdateNotebook, orgID, userID); err != nil {
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
		logger.Errorf("Error updating notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch the updated notebook which includes updated timestamps
	notebook, err = a.db.GetNotebook(notebookID, orgID)
	if err != nil {
		logger.Errorf("Error fetching new notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.sendNotebook(w, r, notebook)
}

// deleteNotebook deletes the notebook with the id
func (a *API) deleteNotebook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := user.LogWith(ctx, logging.Global())

	notebookID, ok := mux.Vars(r)["notebookID"]
	if !ok {
		logger.Errorln("Missing notebookID var")
		http.Error(w, "Missing notebookID", http.StatusBadRequest)
		return
	}

	orgID, _, err := user.ExtractOrgIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID, _, err := user.ExtractUserIDFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := a.requirePermissionTo(ctx, permission.DeleteNotebook, orgID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	err = a.db.DeleteNotebook(notebookID, orgID)
	if err != nil {
		logger.Errorf("Error deleting notebook: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) resolveNotebookReferences(r *http.Request, n *notebooks.Notebook) {
	if err := n.ResolveReferences(r, a.usersClient); err != nil {
		logger := user.LogWith(r.Context(), logging.Global())
		logger.Warnln(err)
	}
}

func (a *API) sendNotebook(w http.ResponseWriter, r *http.Request, notebook notebooks.Notebook) {
	a.resolveNotebookReferences(r, &notebook)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(notebook); err != nil {
		logger := user.LogWith(r.Context(), logging.Global())
		logger.Errorf("Error encoding notebooks: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
