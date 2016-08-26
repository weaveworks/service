package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/render"
	"github.com/weaveworks/service/users/storage"
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
	logins, err := a.db.ListLoginsForUserIDs(currentUser.ID)
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
			logrus.Warningf("Failed fetching %q username for %s: %q", l.Provider, l.ProviderID, err)
		}
		view.Logins = append(view.Logins, v)
	}
	render.JSON(w, http.StatusOK, view)
}

type attachLoginProviderView struct {
	FirstLogin  bool `json:"firstLogin,omitempty"`
	UserCreated bool `json:"userCreated,omitempty"`
	Attach      bool `json:"attach,omitempty"`
}

func (a *API) attachLoginProvider(w http.ResponseWriter, r *http.Request) {
	view := attachLoginProviderView{}
	vars := mux.Vars(r)
	providerID := vars["provider"]
	provider, ok := a.logins.Get(providerID)
	if !ok {
		logrus.Errorf("Login provider not found: %q", providerID)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	id, email, authSession, err := provider.Login(r)
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// Try and find an existing user to attach this login to.
	var u *users.User
	for _, f := range []func() (*users.User, error){
		func() (*users.User, error) {
			// If we have an existing session and an provider, we should use
			// that. This means that we'll associate the provider (if we have
			// one) with the logged in session.
			u, err := a.sessions.Get(r)
			switch err {
			case nil:
				view.Attach = true
			case users.ErrInvalidAuthenticationData:
				err = users.ErrNotFound
			}
			return u, err
		},
		func() (*users.User, error) {
			// If the user has already attached this provider, this is a no-op, so we
			// can just log them in with it.
			return a.db.FindUserByLogin(providerID, id)
		},
		func() (*users.User, error) {
			// Match based on the user's email
			return a.db.FindUserByEmail(email)
		},
	} {
		u, err = f()
		if err == nil {
			break
		} else if err != users.ErrNotFound {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	if u == nil {
		// No matching user found, this must be a first-time-login with this
		// provider, so we'll create an account for them.
		view.UserCreated = true
		u, err = a.db.CreateUser(email)
		if err != nil {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		a.pardotClient.UserCreated(u.Email, u.CreatedAt)
		u, err = a.db.ApproveUser(u.ID)
		if err != nil {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	if err := a.db.AddLoginToUser(u.ID, providerID, id, authSession); err != nil {
		existing, ok := err.(users.AlreadyAttachedError)
		if !ok {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}

		if r.FormValue("force") != "true" {
			render.Error(w, r, existing)
			return
		}
		if err := a.db.DetachLoginFromUser(existing.ID, providerID); err != nil {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
		if err := a.db.AddLoginToUser(u.ID, providerID, id, authSession); err != nil {
			logrus.Error(err)
			render.Error(w, r, users.ErrInvalidAuthenticationData)
			return
		}
	}

	view.FirstLogin = u.FirstLoginAt.IsZero()

	if err := a.updateUserAtLogin(u); err != nil {
		render.Error(w, r, err)
		return
	}

	if err := a.sessions.Set(w, u.ID, providerID); err != nil {
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

	logins, err := a.db.ListLoginsForUserIDs(currentUser.ID)
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

	if err := a.db.DetachLoginFromUser(currentUser.ID, providerID); err != nil {
		render.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type signupView struct {
	MailSent bool   `json:"mailSent"`
	Email    string `json:"email,omitempty"`
	Token    string `json:"token,omitempty"`
}

func (a *API) signup(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var view signupView
	if err := json.NewDecoder(r.Body).Decode(&view); err != nil {
		render.Error(w, r, users.MalformedInputError(err))
		return
	}
	view.MailSent = false
	if view.Email == "" {
		render.Error(w, r, users.ValidationErrorf("Email cannot be blank"))
		return
	}

	user, err := a.db.FindUserByEmail(view.Email)
	if err == users.ErrNotFound {
		user, err = a.db.CreateUser(view.Email)
		// TODO(twilkie) I believe this is redundant, as Approve is also
		// called below
		if err == nil {
			a.pardotClient.UserCreated(user.Email, user.CreatedAt)
			user, err = a.db.ApproveUser(user.ID)
		}
	}
	if err != nil {
		render.Error(w, r, err)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := generateUserToken(a.db, user)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}

	user, err = a.db.ApproveUser(user.ID)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}
	a.pardotClient.UserApproved(user.Email, user.ApprovedAt)

	if a.directLogin {
		view.Token = token
	}

	err = a.emailer.LoginEmail(user, token)
	if err != nil {
		render.Error(w, r, fmt.Errorf("Error sending login email: %s", err))
		return
	}

	view.MailSent = true
	render.JSON(w, http.StatusOK, view)
}

func generateUserToken(db storage.Database, user *users.User) (string, error) {
	token, err := tokens.Generate()
	if err != nil {
		return "", err
	}
	if err := db.SetUserToken(user.ID, token); err != nil {
		return "", err
	}
	return token, nil
}

type loginView struct {
	FirstLogin bool `json:"firstLogin,omitempty"`
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

	u, err := a.db.FindUserByEmail(email)
	if err == users.ErrNotFound {
		err = nil
	}
	if err != nil {
		logrus.Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	if !u.CompareToken(token) {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	if err := a.db.SetUserToken(u.ID, ""); err != nil {
		logrus.Error(err)
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}

	firstLogin := u.FirstLoginAt.IsZero()
	if err := a.updateUserAtLogin(u); err != nil {
		render.Error(w, r, err)
		return
	}

	if err := a.sessions.Set(w, u.ID, ""); err != nil {
		render.Error(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	render.JSON(w, http.StatusOK, loginView{FirstLogin: firstLogin})
}

func (a *API) updateUserAtLogin(u *users.User) error {
	if u.FirstLoginAt.IsZero() {
		if err := a.db.SetUserFirstLoginAt(u.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) logout(_ *users.User, w http.ResponseWriter, r *http.Request) {
	a.sessions.Clear(w)
	render.JSON(w, http.StatusOK, map[string]interface{}{})
}

type publicLookupView struct {
	Email         string    `json:"email,omitempty"`
	Organizations []orgView `json:"organizations,omitempty"`
}

func (a *API) publicLookup(auth Authentication, w http.ResponseWriter, r *http.Request) {
	view := publicLookupView{
		Organizations: []orgView{},
	}
	for _, org := range auth.Organizations {
		view.Organizations = append(view.Organizations, orgView{
			ExternalID:         org.ExternalID,
			Name:               org.Name,
			FirstProbeUpdateAt: render.Time(org.FirstProbeUpdateAt),
			FeatureFlags:       append(org.FeatureFlags, a.forceFeatureFlags...),
		})
	}

	if auth.User != nil {
		view.Email = auth.User.Email
	}

	render.JSON(w, http.StatusOK, view)
}
