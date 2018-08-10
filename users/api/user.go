package api

import (
	"encoding/json"
	"net/http"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

func (a *API) getCurrentUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	resp := users.UserResponse{
		Email:   currentUser.Email,
		Company: currentUser.Company,
		Name:    currentUser.Name,
	}
	render.JSON(w, http.StatusOK, resp)
}

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

	resp := users.UserResponse{
		Email:   user.Email,
		Name:    user.Name,
		Company: user.Company,
	}

	render.JSON(w, http.StatusOK, resp)
}
