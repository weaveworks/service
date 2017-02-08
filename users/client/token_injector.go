package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

var (
	errInvalidRequest = errors.New("GH integration - invalid request")
	errEmptyToken     = errors.New("GH integration - empty token")
)

// Integration represents an API to get GH specific user stuff from the users db
type Integration interface {
	TokenForUser(r *http.Request, provider string) (token string, err error)
}

// TokenRequester Obtains GH tokens
type TokenRequester struct {
	URL          string
	client       http.Client
	UserIDHeader string
}

// ProviderToken is used for passing a token around
type ProviderToken struct {
	Token string `json:"token"`
}

// TokenForUser get a token for a user
func (t *TokenRequester) TokenForUser(r *http.Request, provider string) (token string, err error) {
	// This middleware is ran after the authenticate user middleware
	// so userID information is located in the header.
	userID := r.Header.Get(t.UserIDHeader)
	if userID == "" {
		err = errInvalidRequest
		return
	}

	u, err := url.Parse(t.URL)
	if err != nil {
		return
	}
	u.Path = fmt.Sprintf("/private/api/users/%s/logins/%s/token", userID, provider)

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return
	}

	return t.decodeToken(t.doTokenRequest(req))
}

func (t *TokenRequester) doTokenRequest(r *http.Request) (body io.ReadCloser, err error) {
	resp, err := t.client.Do(r)
	if err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(resp.Body)
		err = &APIError{
			StatusCode: resp.StatusCode,
			Status:     fmt.Sprint(resp.StatusCode),
			Body:       string(b),
		}
		return
	}
	body = resp.Body
	return
}

func (t *TokenRequester) decodeToken(body io.ReadCloser, err error) (string, error) {
	if err != nil {
		return "", err
	}
	defer body.Close()
	var tok ProviderToken
	err = json.NewDecoder(body).Decode(&tok)
	if err != nil {
		return "", err
	}
	if tok.Token == "" {
		return "", errEmptyToken
	}
	return tok.Token, nil
}

// GHIntegrationMiddleware will inject the GH token into the header of the request
type GHIntegrationMiddleware struct {
	T Integration
}

// Wrap implements middleware.Interface
func (t GHIntegrationMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, err := t.T.TokenForUser(r, "github")
		if err != nil {
			apiErr, isAPIErr := err.(*APIError)
			if isAPIErr {
				w.WriteHeader(apiErr.StatusCode)
			} else {
				w.WriteHeader(http.StatusUnprocessableEntity)
			}
			fmt.Fprintf(w, err.Error())
			return
		}
		// Set token in header
		r.Header.Set("GithubToken", tok)
		next.ServeHTTP(w, r)
	})
}
