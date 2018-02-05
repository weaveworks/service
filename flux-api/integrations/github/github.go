package github

import (
	"fmt"

	gh "github.com/google/go-github/github"
	"github.com/weaveworks/flux/http/httperror"

	"net/http"

	"golang.org/x/oauth2"
)

var (
	defaultDeployKeyName = "flux-generated"
	errUnauthorized      = httperror.APIError{
		Body: "Unable to list deploy keys. Permission denied. Check user token.",
	}
	errNotFound = httperror.APIError{
		Body: "Cannot find owner or repository. Check spelling.",
	}
	errGeneric = httperror.APIError{
		Body: "Unable to perform GH action. Check error message.",
	}
)

// Github is a Github API client.
type Github struct {
	client *gh.Client
}

// NewGithubClient instantiates a GH client from a provided OAuth token.
func NewGithubClient(token string) *Github {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return &Github{
		client: gh.NewClient(tc),
	}
}

// InsertDeployKey will create a new deploy key titled `deployKeyName`
// (or defaultDeployKeyName if that argument is empty) for the given
// owner, repo, token, containing the public key `deployKey`.  If a
// key already exists with that title it will be deleted first.
func (g *Github) InsertDeployKey(ownerName string, repoName string, deployKey, deployKeyName string) error {
	if deployKeyName == "" {
		deployKeyName = defaultDeployKeyName
	}
	// Get list of keys
	keys, resp, err := g.client.Repositories.ListKeys(ownerName, repoName, nil)
	if err != nil {
		return parseError(resp, err)
	}
	for _, k := range keys {
		// If key already exists, delete
		if *k.Title == deployKeyName {
			resp, err := g.client.Repositories.DeleteKey(ownerName, repoName, *k.ID)
			if err != nil {
				return parseError(resp, err)
			}
			break
		}
	}

	// Create new key
	key := gh.Key{
		Title: &deployKeyName,
		Key:   &deployKey,
	}
	_, resp, err = g.client.Repositories.CreateKey(ownerName, repoName, &key)
	if err != nil {
		return parseError(resp, err)
	}
	return nil
}

func populateError(err httperror.APIError, resp *gh.Response) *httperror.APIError {
	err.StatusCode = resp.StatusCode
	err.Status = resp.Status
	return &err
}

func parseError(resp *gh.Response, err error) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return populateError(errUnauthorized, resp)
	case http.StatusNotFound:
		return populateError(errNotFound, resp)
	default:
		e := populateError(errGeneric, resp)
		e.Body = fmt.Sprintf("%s - %s", e.Body, err.Error())
		return e
	}
}
