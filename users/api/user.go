package api

import (
	"encoding/json"
	"net/http"

	"github.com/microcosm-cc/bluemonday"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

var stripHTML = bluemonday.StrictPolicy().Sanitize

func (a *API) getCurrentUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	resp := users.UserResponse{
		Email:     currentUser.Email,
		Company:   currentUser.Company,
		Name:      currentUser.Name,
		FirstName: currentUser.FirstName,
		LastName:  currentUser.LastName,
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

	update.Name = stripHTML(update.Name)
	update.Company = stripHTML(update.Company)
	update.FirstName = stripHTML(update.FirstName)
	update.LastName = stripHTML(update.LastName)

	user, err := a.db.UpdateUser(r.Context(), currentUser.ID, update)
	if err != nil {
		renderError(w, r, err)
		return
	}

	resp := users.UserResponse{
		Email:     user.Email,
		Name:      user.Name,
		Company:   stripHTML(user.Company),
		FirstName: stripHTML(user.FirstName),
		LastName:  stripHTML(user.LastName),
	}

	render.JSON(w, http.StatusOK, resp)
}
