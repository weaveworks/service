package dao

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"golang.org/x/oauth2"
)

// UsersClient is an interface for users service's clients.
type UsersClient interface {
	// GoogleOAuthToken returns the Google OAuth token for the specified user.
	GoogleOAuthToken(userID string) (*oauth2.Token, error)
}

// UsersHTTPClient is a HTTP implementation of UsersClient.
type UsersHTTPClient struct {
	UsersHostPort string
}

// GoogleOAuthToken returns the Google OAuth token for the specified user.
func (c UsersHTTPClient) GoogleOAuthToken(userID string) (*oauth2.Token, error) {
	logger := log.WithField("user_id", userID)
	resp, err := http.Get(fmt.Sprintf("%s/admin/users/users/%s/logins/google/token", c.UsersHostPort, userID))
	if err != nil {
		logger.Errorf("Failed to get token from users service: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorf("Failed to read response from users service: %v", err)
		return nil, err
	}

	token := &struct {
		AccessToken string `json:"token"`
	}{}
	if err = json.Unmarshal(body, token); err != nil {
		logger.Errorf("Failed to deserialise token from users service: %v", err)
		return nil, err
	}
	return &oauth2.Token{AccessToken: token.AccessToken}, nil
}

// UsersNoOpClient is a no-op implementation of UsersClient.
// This implementation is mostly useful for testing.
type UsersNoOpClient struct {
}

// GoogleOAuthToken returns an arbitrary string and a nil error.
func (c UsersNoOpClient) GoogleOAuthToken(userID string) (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "<token>"}, nil
}
