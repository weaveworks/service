package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/tokens"
)

func (a *API) lookupOrgHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	orgExternalID := mux.Vars(r)["orgExternalID"]

	view, err := a.grpc.LookupOrg(r.Context(), &users.LookupOrgRequest{cookie, orgExternalID})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func (a *API) lookupAdminHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	view, err := a.grpc.LookupAdmin(r.Context(), &users.LookupAdminRequest{
		Cookie: cookie,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func (a *API) lookupUsingTokenHandler(w http.ResponseWriter, r *http.Request) {
	token, ok := tokens.ExtractToken(r)
	if !ok {
		render.Error(w, r, users.NewInvalidAuthenticationDataError(fmt.Errorf("token extraction failure")))
		return
	}

	view, err := a.grpc.LookupUsingToken(r.Context(), &users.LookupUsingTokenRequest{
		Token: token,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func (a *API) lookupUserHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := sessions.Extract(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	view, err := a.grpc.LookupUser(r.Context(), &users.LookupUserRequest{
		Cookie: cookie,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, view)
}
