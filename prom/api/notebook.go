package api

import (
	"encoding/json"
	"net/http"

	"github.com/weaveworks/common/user"
	"google.golang.org/appengine/log"
)

// getAllNotebooks returns all of the notebooks for an instance.
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
