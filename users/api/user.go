package api

import (
	"encoding/json"
	"net/http"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

func (a *API) updateUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var update *users.UserUpdate

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}

	user, err := a.db.UpdateUser(r.Context(), currentUser.ID, update)
	if err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, user)
}
