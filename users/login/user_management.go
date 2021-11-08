package login

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gopkg.in/auth0.v5"
	"gopkg.in/auth0.v5/management"

	"github.com/weaveworks/service/common"
)

// NewAuth0Management initializes a auth0 management API client
func NewAuth0Management(domain string, clientID string, clientSecret string) (*management.Management, error) {
	api, err := management.New(domain, management.WithClientCredentials(clientID, clientSecret))
	if err != nil {
		return nil, err
	}

	return api, nil
}

// InviteUser creates a new user, and send them an invite to join a team. This is done
// by first creating the user, and then requesting a passwordless login.
// The user is created with a set of user_metadata that shows the user has been
// invited, to what, and by whom.
func (p *Auth0Provider) InviteUser(email string, inviter string, teamName string) error {
	err := p.api.User.Create(&management.User{
		Connection:    auth0.String("email"),
		Email:         &email,
		EmailVerified: auth0.Bool(true), // Is a lie, but I can't get `verify_email: false` to work
		UserMetadata: map[string]interface{}{
			"invited_to": teamName,
			"invited_by": inviter,
		},
	})

	if err != nil {
		return err
	}

	err = p.PasswordlessLogin(nil, email)
	return err
}

// PasswordlessLogin initializes a passwordless login. This is pretty much the
// same request as with a regular social login, except to a different endpoint
func (p *Auth0Provider) PasswordlessLogin(r *http.Request, email string) error {
	authParams := map[string]string{
		"scope":         "openid email profile",
		"response_type": "code",
	}
	if r != nil {
		token := csrfToken(r)
		state := common.FlattenQueryParams(r.URL.Query())
		state["token"] = token
		authParams["state"] = p.encodeState(state)
	} else {
		// Bouncing the user through the verify endpoint sends us back to auth0
		// and makes them return with proper CSRF protection
		verify, _ := p.siteDomain.Parse("/api/users/verify")
		authParams["redirect_uri"] = verify.String()
	}
	payload := map[string]interface{}{
		"client_id":     p.clientID,
		"client_secret": p.clientSecret,
		"connection":    "email",
		"email":         email,
		"send":          "link",
		"authParams":    authParams,
	}

	message, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	passwordlessURL, _ := p.authDomain.Parse("passwordless/start")
	req, err := http.NewRequest("POST", passwordlessURL.String(), bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Couldn't create invite: %v", string(body))
	}
	return nil
}

// UpdateClaims updates our user in auth0's database.
// You will get an error if you try to update anything but UserMetadata
// for any user that isn't with the email provider
func (p *Auth0Provider) UpdateClaims(ctx context.Context, claims Claims, session json.RawMessage) error {
	user := management.User{
		UserMetadata: map[string]interface{}{
			"invited_to":   nil, // Make sure we clear invite flags - this happens after first login
			"invited_by":   nil,
			"company_name": claims.UserMetadata.CompanyName,
		},
	}

	if claims.Name != "" {
		user.Name = &claims.Name
	}
	if claims.FamilyName != "" {
		user.FamilyName = &claims.FamilyName
	}
	if claims.GivenName != "" {
		user.GivenName = &claims.GivenName
	}

	err := p.api.User.Update(claims.ID, &user)

	if err != nil {
		return err
	}

	return nil
}

// GetAccessToken returns an access token that can be used to talk directly to the login
// provider. E.g. call this on a github login to get access to the github API
func (p *Auth0Provider) GetAccessToken(loginID string) (*string, error) {
	u, err := p.api.User.Read(loginID)
	if err != nil {
		return nil, err
	}

	for _, identity := range u.Identities {
		return identity.AccessToken, nil
	}
	return nil, fmt.Errorf("Couldn't find any user identities")
}
