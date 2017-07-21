package api

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/tokens"
)

type loginProvidersView struct {
	Logins []loginProviderView `json:"logins"`
}

type loginProviderView struct {
	ID   string     `json:"id"`
	Name string     `json:"name"` // Human-readable name of this provider
	Link login.Link `json:"link"` // HTML Attributes for the link to start this provider flow
}

func (a *API) listLoginProviders(w http.ResponseWriter, r *http.Request) {
	view := loginProvidersView{}
	a.logins.ForEach(func(id string, p login.Provider) {
		v := loginProviderView{
			ID:   id,
			Name: p.Name(),
		}
		if link, ok := p.Link(r); ok {
			v.Link = link
		}
		view.Logins = append(view.Logins, v)
	})
	render.JSON(w, http.StatusOK, view)
}

type attachedLoginProvidersView struct {
	Logins []attachedLoginProviderView `json:"logins"`
}

type attachedLoginProviderView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LoginID  string `json:"loginID,omitempty"`
	Username string `json:"username,omitempty"`
}

// List all the login providers currently attached to the current user. Used by
// the /account page to determine which login providers are currently attached.
func (a *API) listAttachedLoginProviders(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	view := attachedLoginProvidersView{}
	logins, err := a.db.ListLoginsForUserIDs(r.Context(), currentUser.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	for _, l := range logins {
		p, ok := a.logins.Get(l.Provider)
		if !ok {
			continue
		}

		v := attachedLoginProviderView{
			ID:      l.Provider,
			Name:    p.Name(),
			LoginID: l.ProviderID,
		}
		var err error
		v.Username, err = p.Username(l.Session)
		if err != nil {
			logging.With(r.Context()).Warningf("Failed fetching %q username for %s: %q", l.Provider, l.ProviderID, err)
		}
		view.Logins = append(view.Logins, v)
	}
	render.JSON(w, http.StatusOK, view)
}

type attachLoginProviderView struct {
	FirstLogin   bool   `json:"firstLogin,omitempty"`
	UserCreated  bool   `json:"userCreated,omitempty"`
	Attach       bool   `json:"attach,omitempty"`
	Email        string `json:"email"`
	MunchkinHash string `json:"munchkinHash"`
}

func (a *API) attachLoginProvider(w http.ResponseWriter, r *http.Request) {
	view := attachLoginProviderView{}
	vars := mux.Vars(r)
	providerID := vars["provider"]
	provider, ok := a.logins.Get(providerID)
	if !ok {
		logging.With(r.Context()).Errorf("Login provider not found: %q", providerID)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	id, email, authSession, err := provider.Login(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if email == "" {
		logging.With(r.Context()).Errorf("Login provider returned blank email: %q", providerID)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
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
			return a.db.FindUserByID(r.Context(), session.UserID)
		},
		func() (*users.User, error) {
			// If the user has already attached this provider, this is a no-op, so we
			// can just log them in with it.
			return a.db.FindUserByLogin(r.Context(), providerID, id)
		},
		func() (*users.User, error) {
			// Match based on the user's email
			return a.db.FindUserByEmail(r.Context(), email)
		},
	} {
		u, err = f()
		if err == nil {
			break
		} else if err != users.ErrNotFound {
			logging.With(r.Context()).Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	if u == nil {
		// No matching user found, this must be a first-time-login with this
		// provider, so we'll create an account for them.
		view.UserCreated = true
		u, err = a.db.CreateUser(r.Context(), email)
		if err != nil {
			logging.With(r.Context()).Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		a.marketingQueues.UserCreated(u.Email, u.CreatedAt)
	}

	if err := a.db.AddLoginToUser(r.Context(), u.ID, providerID, id, authSession); err != nil {
		existing, ok := err.(users.AlreadyAttachedError)
		if !ok {
			logging.With(r.Context()).Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}

		if r.FormValue("force") != "true" {
			render.Error(w, r, existing)
			return
		}
		if err := a.db.DetachLoginFromUser(r.Context(), existing.ID, providerID); err != nil {
			logging.With(r.Context()).Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		if err := a.db.AddLoginToUser(r.Context(), u.ID, providerID, id, authSession); err != nil {
			logging.With(r.Context()).Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	view.FirstLogin = u.FirstLoginAt.IsZero()
	view.Email = email
	view.MunchkinHash = a.MunchkinHash(email)

	if err := a.UpdateUserAtLogin(r.Context(), u); err != nil {
		render.Error(w, r, err)
		return
	}

	impersonatingUserID := "" // Logging in via provider credentials => cannot be impersonating
	if err := a.sessions.Set(w, u.ID, impersonatingUserID); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	render.JSON(w, http.StatusOK, view)
}

func (a *API) detachLoginProvider(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerID := vars["provider"]
	provider, ok := a.logins.Get(providerID)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	logins, err := a.db.ListLoginsForUserIDs(r.Context(), currentUser.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	for _, login := range logins {
		if login.Provider != providerID {
			continue
		}
		if err := provider.Logout(login.Session); err != nil {
			render.Error(w, r, err)
			return
		}
	}

	if err := a.db.DetachLoginFromUser(r.Context(), currentUser.ID, providerID); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SignupView is both the input and output from Signup.  Eugh.
type SignupView struct {
	MailSent bool   `json:"mailSent"`
	Email    string `json:"email,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view SignupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}

	if _, err := a.Signup(r.Context(), &view); err != nil {
		render.Error(w, r, err)
	}

	render.JSON(w, http.StatusOK, view)
}

// Signup creates a new user
func (a *API) Signup(ctx context.Context, view *SignupView) (*users.User, error) {
	view.MailSent = false
	if view.Email == "" {
		return nil, users.ValidationErrorf("Email cannot be blank")
	}

	user, err := a.db.FindUserByEmail(ctx, view.Email)
	if err == users.ErrNotFound {
		user, err = a.db.CreateUser(ctx, view.Email)
		if err == nil {
			a.marketingQueues.UserCreated(user.Email, user.CreatedAt)
		}
	}
	if err != nil {
		return nil, err
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := a.generateUserToken(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("Error sending login email: %s", err)
	}

	if a.directLogin {
		view.Token = token
	}

	err = a.emailer.LoginEmail(user, token)
	if err != nil {
		return nil, fmt.Errorf("Error sending login email: %s", err)
	}

	view.MailSent = true
	return user, nil
}

func (a *API) generateUserToken(ctx context.Context, user *users.User) (string, error) {
	token, err := tokens.Generate()
	if err != nil {
		return "", err
	}
	if err := a.db.SetUserToken(ctx, user.ID, token); err != nil {
		return "", err
	}
	return token, nil
}

type loginView struct {
	FirstLogin   bool   `json:"firstLogin,omitempty"`
	Email        string `json:"email"`
	MunchkinHash string `json:"munchkinHash"`
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token := r.FormValue("token")
	switch {
	case email == "":
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	case token == "":
		render.Error(w, r, users.ValidationErrorf("Token cannot be blank"))
		return
	}

	u, err := a.db.FindUserByEmail(r.Context(), email)
	if err == users.ErrNotFound {
		err = nil
	}
	if err != nil {
		logging.With(r.Context()).Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	if err := a.db.SetUserToken(r.Context(), u.ID, ""); err != nil {
		logging.With(r.Context()).Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	firstLogin := u.FirstLoginAt.IsZero()
	if err := a.UpdateUserAtLogin(r.Context(), u); err != nil {
		render.Error(w, r, err)
		return
	}

	impersonatingUserID := "" // Direct login => cannot be impersonating
	if err := a.sessions.Set(w, u.ID, impersonatingUserID); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	render.JSON(w, http.StatusOK, loginView{
		FirstLogin:   firstLogin,
		Email:        email,
		MunchkinHash: a.MunchkinHash(email),
	})
}

// UpdateUserAtLogin sets u.FirstLoginAt if not already set
func (a *API) UpdateUserAtLogin(ctx context.Context, u *users.User) error {
	if u.FirstLoginAt.IsZero() {
		if err := a.db.SetUserFirstLoginAt(ctx, u.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w)
	render.JSON(w, http.StatusOK, map[string]interface{}{})
}

type publicLookupView struct {
	Email         string    `json:"email,omitempty"`
	Organizations []OrgView `json:"organizations,omitempty"`
	MunchkinHash  string    `json:"munchkinHash"`
	IntercomHash  string    `json:"intercomHash"`
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

// IntercomHash caclulates the hash for Intercom's user verification.
// See https://docs.intercom.com/configure-intercom-for-your-product-or-site/staying-secure/enable-identity-verification-on-your-web-product for details.
func (a *API) IntercomHash(email string) string {
	h := hmac.New(sha256.New, []byte(a.intercomHashKey))
	h.Write([]byte(email))
	return hex.EncodeToString(h.Sum(nil))
}

func (a *API) publicLookup(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	organizations, err := a.db.ListOrganizationsForUserIDs(r.Context(), currentUser.ID)
	if err != nil {
		render.Error(w, r, err)
		return
	}
	view := publicLookupView{
		Email:        currentUser.Email,
		MunchkinHash: a.MunchkinHash(currentUser.Email),
		IntercomHash: a.IntercomHash(currentUser.Email),
	}
	for _, org := range organizations {
		view.Organizations = append(view.Organizations, OrgView{
			ExternalID:     org.ExternalID,
			Name:           org.Name,
			FeatureFlags:   append(org.FeatureFlags, a.forceFeatureFlags...),
			DenyUIFeatures: org.DenyUIFeatures,
			DenyTokenAuth:  org.DenyTokenAuth,
		})
	}
	render.JSON(w, http.StatusOK, view)
}
