package api

import (
	"encoding/json"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/prom"

	"github.com/satori/go.uuid"
)

// getAllNotebooks returns all of the notebooks for an instance
func (a *API) getAllNotebooks(w http.ResponseWriter, r *http.Request) {
	orgID, _, err := user.ExtractFromHTTPRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	notebooks, err := a.db.GetAllNotebooks(orgID)
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

// CreateNotebook describes the structure of create notebook requests
type CreateNotebook struct {
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

	var input CreateNotebook
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
