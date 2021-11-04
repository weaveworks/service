package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/microcosm-cc/bluemonday"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/common/validation"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
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

func validateNames(name, first, last, company string) error {
	if !validation.ValidateName(name) {
		return users.ValidationErrorf("Please provide a valid name")
	}
	if !validation.ValidateName(first) {
		return users.ValidationErrorf("Please provide a valid first name")
	}
	if !validation.ValidateName(last) {
		return users.ValidationErrorf("Please provide a valid last name")
	}
	if !validation.ValidateName(company) {
		return users.ValidationErrorf("Please provide a valid company name")
	}
	return nil
}

func (a *API) updateUser(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var update *users.UserUpdate

	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}
	if err := validateNames(update.Name, update.FirstName, update.LastName, update.Company); err != nil {
		renderError(w, r, err)
		return
	}

	session, _ := a.sessions.Get(r)
	l, err := a.db.GetLogin(r.Context(), session.Provider, session.LoginID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	update.Name = strings.TrimSpace(stripHTML(update.Name))
	update.Company = strings.TrimSpace(stripHTML(update.Company))
	update.FirstName = strings.TrimSpace(stripHTML(update.FirstName))
	update.LastName = strings.TrimSpace(stripHTML(update.LastName))

	claims := login.Claims{ID: session.LoginID}
	// We can only update built-in fields for database backed logins
	if session.Provider == "email" {
		claims.Name = update.Name
		claims.GivenName = update.FirstName
		claims.FamilyName = update.LastName
	}
	// We can always update metadata
	claims.UserMetadata.CompanyName = update.Company

	err = a.logins.UpdateClaims(r.Context(), claims, l.Session)
	if err != nil {
		renderError(w, r, err)
		return
	}

	_, err = a.db.UpdateUser(r.Context(), currentUser.ID, update)
	if err != nil {
		renderError(w, r, err)
		return
	}

	resp := users.UserResponse{
		Email:     claims.Email,
		Name:      claims.Name,
		Company:   claims.UserMetadata.CompanyName,
		FirstName: claims.GivenName,
		LastName:  claims.FamilyName,
	}

	render.JSON(w, http.StatusOK, resp)
}
