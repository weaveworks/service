package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
	commonuser "github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/common/validation"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
)

type loginProvidersView struct {
	Logins []loginProviderView `json:"logins"`
}

type link struct {
	Href string `json:"href"`
}

type loginProviderView struct {
	ID   string `json:"id"`
	Name string `json:"name"` // Human-readable name of this provider
	Link link   `json:"link"`
}

func (a *API) listLoginProviders(w http.ResponseWriter, r *http.Request) {
	view := loginProvidersView{}
	query := r.URL.Query()
	for _, c := range []struct {
		name  string
		label string
	}{
		{"google", "Google"},
		{"github", "GitHub"},
	} {
		query.Set("connection", c.name)
		view.Logins = append(view.Logins, loginProviderView{c.name, c.label, link{"/api/users/verify?" + query.Encode()}})
	}
	render.JSON(w, http.StatusOK, view)
}

type attachLoginProviderView struct {
	FirstLogin   bool              `json:"firstLogin,omitempty"`
	UserCreated  bool              `json:"userCreated,omitempty"`
	Attach       bool              `json:"attach,omitempty"`
	Email        string            `json:"email"`
	MunchkinHash string            `json:"munchkinHash"`
	QueryParams  map[string]string `json:"queryParams,omitempty"`
}

// attachLoginProvider is used for oauth login or signup
func (a *API) attachLoginProvider(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := commonuser.LogWith(ctx, logging.Global())
	view := attachLoginProviderView{}
	vars := mux.Vars(r)
	providerID := vars["provider"]
	claims, authSession, extraState, err := a.logins.Login(r)
	connectionName := extraState["connection"]
	delete(extraState, "connection")
	view.QueryParams = extraState
	if err != nil {
		renderError(w, r, err)
		return
	}
	if claims.Email == "" {
		logger.Errorf("Login provider returned blank email: %q", providerID)
		renderError(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	if !validation.ValidateEmail(claims.Email) {
		logger.Errorf("Login provider returned an invalid email: %q, %v", providerID, claims.Email)
		renderError(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	if providerID == "auth0" {
		// auth0 proxys other federated auth - we still need the source, e.g.
		// "can we get access to github using this login"
		providerID = connectionName
	}

	// Try and find an existing user to attach this login to.
	var u *users.User
	for _, f := range []func() (*users.User, error){
		func() (*users.User, error) {
			// If we have an existing session and an provider, we should use
			// that. This means that we'll associate the provider (if we have
			// one) with the logged in session.
			session, err := a.sessions.Get(r)
			switch err {
			case nil:
				view.Attach = true
			case users.ErrInvalidAuthenticationData:
				return nil, users.ErrNotFound
			default:
				return nil, err
			}
			return a.db.FindUserByID(ctx, session.UserID)
		},
		func() (*users.User, error) {
			// If the user has already attached this provider, this is a no-op, so we
			// can just log them in with it.
			return a.db.FindUserByLogin(ctx, providerID, claims.ID)
		},
		func() (*users.User, error) {
			// If the user had already attached this provider before auth0 migration,
			// we can just log them in with it. This might be necessary if github isn't
			// returning an email.
			parts := strings.SplitN(claims.ID, "|", 2)
			if len(parts) != 2 {
				return nil, users.ErrNotFound
			}
			oldID := parts[1]
			return a.db.FindUserByLogin(ctx, providerID, oldID)
		},
		func() (*users.User, error) {
			// Match based on the user's email
			return a.db.FindUserByEmail(ctx, claims.Email)
		},
	} {
		u, err = f()
		if err == nil {
			break
		} else if err != users.ErrNotFound {
			logger.Errorln(err)
			renderError(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	if u == nil {
		// No matching user found, this must be a first-time-login with this
		// provider, so we'll create an account for them.
		view.UserCreated = true
		givenName, familyName := getNameFromClaims(claims)
		userUpdate := users.UserUpdate{
			Name:      claims.Name,
			FirstName: givenName,
			LastName:  familyName,
			Company:   claims.UserMetadata.CompanyName,
		}
		u, err = a.db.CreateUser(ctx, claims.Email, &userUpdate)
		if err != nil {
			logger.Errorln(err)
			renderError(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		a.marketingQueues.UserCreated(u.Email, u.FirstName, u.LastName, u.Company, u.CreatedAt, extraState)
	} else {
		a.marketingQueues.UserAccess(u.Email, time.Now())
	}

	if err := a.db.AddLoginToUser(ctx, u.ID, providerID, claims.ID, authSession); err != nil {
		existing, ok := err.(*users.AlreadyAttachedError)
		if !ok {
			logger.Errorln(err)
			renderError(w, r, users.ErrInvalidAuthenticationData)
			return
		}

		if r.FormValue("force") != "true" {
			renderError(w, r, existing)
			return
		}
		if err := a.db.DetachLoginFromUser(ctx, existing.ID, providerID); err != nil {
			logger.Errorln(err)
			renderError(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		if err := a.db.AddLoginToUser(ctx, u.ID, providerID, claims.ID, authSession); err != nil {
			logger.Errorln(err)
			renderError(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	view.FirstLogin = u.FirstLoginAt.IsZero()
	view.Email = claims.Email
	view.MunchkinHash = a.MunchkinHash(claims.Email)

	if err := a.UpdateUserAtLogin(ctx, u); err != nil {
		renderError(w, r, err)
		return
	}

	impersonatingUserID := "" // Logging in via provider credentials => cannot be impersonating
	if err := a.sessions.Set(w, r, providerID, claims.ID, u.ID, impersonatingUserID); err != nil {
		renderError(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func getNameFromClaims(claims *login.Claims) (string, string) {
	var givenName, familyName string
	if claims.GivenName != "" || claims.FamilyName != "" {
		givenName = claims.GivenName
		familyName = claims.FamilyName
	} else if strings.Contains(claims.Name, " ") {
		// Western-centric fallback - github only provides full name
		names := strings.SplitN(claims.Name, " ", 2)
		givenName, familyName = names[0], names[1]
	}

	return givenName, familyName
}

func (a *API) detachLoginProvider(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["provider"]

	if err := a.db.DetachLoginFromUser(r.Context(), currentUser.ID, providerID); err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// EmailLoginRequest is the message sent to initiate a signup request
type EmailLoginRequest struct {
	Email     string `json:"email,omitempty"`
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Company   string `json:"company,omitempty"`
	// QueryParams are url query params from the login page, we pass them on because they are used for tracking
	QueryParams map[string]string `json:"queryParams,omitempty"`
}

// EmailLoginResponse is the message sent as the result of a signup request
type EmailLoginResponse struct {
	Email string `json:"email,omitempty"`
	// QueryParams are url query params from the login page, we pass them on because they are used for tracking
	QueryParams map[string]string `json:"queryParams,omitempty"`
}

func (a *API) emailLogin(w http.ResponseWriter, r *http.Request) {
	logger := commonuser.LogWith(r.Context(), logging.Global())
	defer r.Body.Close()
	var input EmailLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}

	if a.createAdminUsers {
		// This path is enabled for local development only
		u, err := makeLocalTestUser(
			r.Context(),
			a,
			input.Email,
			"local-test",
			"Local Test Instance",
			"local-test-token",
			"Local Team",
		)
		if err != nil {
			logger.Errorf("Error setting up local test user: %w", err)
			renderError(w, r, err)
			return
		}
		// TODO: does this make myself get logged in now?
		a.sessions.Set(w, r, "dummy", input.Email, u.ID, "")
		render.JSON(w, http.StatusOK, EmailLoginResponse{
			Email: input.Email,
		})
		return
	}

	resp, err := a.EmailLogin(r, input)
	if err != nil {
		renderError(w, r, err)
	} else {
		render.JSON(w, http.StatusOK, resp)
	}
}

// EmailLogin creates a new user (but will also allow an existing user to log in)
// NB: this is used only for email signups, not oauth signups
func (a *API) EmailLogin(r *http.Request, req EmailLoginRequest) (*EmailLoginResponse, error) {
	if req.Email == "" {
		return nil, users.ValidationErrorf("Email cannot be blank")
	}
	email := strings.TrimSpace(req.Email)

	if !validation.ValidateEmail(email) {
		return nil, users.ValidationErrorf("Please provide a valid email")
	}

	resp := EmailLoginResponse{
		Email: email,
	}

	err := a.logins.PasswordlessLogin(r, email)
	if err != nil {
		return nil, fmt.Errorf("Error sending login email: %s", err)
	}

	return &resp, nil
}

// healthCheck handles a very simple health check
func (a *API) healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// verify ensures that we're logged in, redirecting us to the login page if not,
// and redirecting us back to whence we came if we are
func (a *API) verify(w http.ResponseWriter, r *http.Request) {
	connection := r.FormValue("connection")
	returnURL := r.FormValue("next")
	var loginURL string
	if r.FormValue("code") != "" && connection == "" {
		connection = "email"
	}
	if connection != "" {
		loginURL = a.logins.LoginURL(r, connection)
	} else {
		loginURL = "/login"
		if returnURL != "" {
			loginURL = loginURL + "?next=" + url.QueryEscape(returnURL)
		} else {
			returnURL = "/"
		}
	}

	var finalURL string
	session, err := a.sessions.Get(r)
	if err != nil {
		finalURL = loginURL
	} else if session.UserID == "" {
		finalURL = loginURL
	} else if connection != "" && connection != session.Provider {
		finalURL = loginURL
	} else {
		finalURL = returnURL
	}

	http.Redirect(w, r, finalURL, http.StatusSeeOther)
}

// UpdateUserAtLogin sets u.FirstLoginAt if not already set
func (a *API) UpdateUserAtLogin(ctx context.Context, u *users.User) error {
	return a.db.SetUserLastLoginAt(ctx, u.ID)
}

// logs you out from weave cloud (clears cookie) and sends you to the provider
// logout page
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w, r)

	http.Redirect(w, r, a.logins.LogoutURL(r), http.StatusSeeOther)
	render.JSON(w, http.StatusOK, map[string]interface{}{})
}

type publicLookupView struct {
	Email         string    `json:"email,omitempty"`
	Name          string    `json:"name,omitempty"`
	FirstName     string    `json:"firstName,omitempty"`
	LastName      string    `json:"lastName,omitempty"`
	Company       string    `json:"company,omitempty"`
	FirstLoginAt  string    `json:"firstLoginAt,omitempty"`
	Organizations []OrgView `json:"organizations"`
	MunchkinHash  string    `json:"munchkinHash"`
}

// MunchkinHash caclulates the hash for Marketo's Munchkin tracking code.
// See http://developers.marketo.com/javascript-api/lead-tracking/api-reference/#munchkin_associatelead for details.
// Public for testing.
func (a *API) MunchkinHash(email string) string {
	h := sha1.New()
	h.Write([]byte(a.marketoMunchkinKey))
	h.Write([]byte(email))
	return hex.EncodeToString(h.Sum(nil))
}

func (a *API) publicLookup(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), currentUser.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	view := publicLookupView{
		Email:         currentUser.Email,
		Name:          currentUser.Name,
		FirstName:     currentUser.FirstName,
		LastName:      currentUser.LastName,
		Company:       currentUser.Company,
		FirstLoginAt:  currentUser.FormatFirstLoginAt(),
		MunchkinHash:  a.MunchkinHash(currentUser.Email),
		Organizations: make([]OrgView, 0),
	}
	for _, org := range organizations {
		view.Organizations = append(view.Organizations, a.createOrgView(r.Context(), currentUser, org))
	}
	render.JSON(w, http.StatusOK, view)
}

func makeLocalTestUser(ctx context.Context, a *API, email string, instanceID, instanceName, token, teamName string) (*users.User, error) {
	if u, err := a.db.FindUserByEmail(ctx, email); err == nil {
		// User already exists, just return them
		return u, nil
	}

	user, err := a.db.CreateUser(ctx, email, &users.UserUpdate{
		Name:      "Testy McTestface",
		FirstName: "Testy",
		LastName:  "McTestface",
		Company:   "Acme Inc.",
	})
	if err != nil {
		return nil, err
	}

	if err := a.UpdateUserAtLogin(ctx, user); err != nil {
		return nil, err
	}

	if err := a.MakeUserAdmin(ctx, user.ID, true); err != nil {
		return nil, err
	}

	now := time.Now()
	if err := a.CreateOrg(ctx, user, OrgView{
		ExternalID:     instanceID,
		Name:           instanceName,
		ProbeToken:     token,
		TrialExpiresAt: user.TrialExpiresAt(),
		TeamName:       teamName,
	}, now); err != nil {
		return nil, err
	}

	if err := a.SetOrganizationFirstSeenConnectedAt(ctx, instanceID, &now); err != nil {
		return nil, err
	}

	return user, nil
}
