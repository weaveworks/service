package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

type apiTokensView struct {
	APITokens []*users.APIToken `json:"tokens"`
}

func (a *API) listAPITokens(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	var (
		view apiTokensView
		err  error
	)
	view.APITokens, err = a.db.ListAPITokensForUserIDs(currentUser.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if view.APITokens == nil {
		view.APITokens = []*users.APIToken{}
	}
	render.JSON(w, http.StatusOK, view)
}

type createAPITokenView struct {
	Description string `json:"description"`
}

func (a *API) createAPIToken(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view createAPITokenView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}

	token, err := a.db.CreateAPIToken(currentUser.ID, view.Description)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	render.JSON(w, http.StatusCreated, token)
}

func (a *API) deleteAPIToken(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	if err := a.db.DeleteAPIToken(currentUser.ID, mux.Vars(r)["token"]); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
