package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/db/memory"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/templates"
)

var (
	ghToken   = "e12eb509a297f56dcc77c86ec9e44369080698a6"
	ghSession = []byte(`{"token": {"expiry": "0001-01-01T00:00:00Z", "token_type": "bearer", "access_token": "` + ghToken + `"}}`)
	ctx       = context.Background()
	smtp      = &emailer.SMTPEmailer{
		Templates:   templates.MustNewEngine("../templates"),
		Domain:      "https://weave.test",
		FromAddress: "from@weave.test",
	}
)

func setup(d db.DB) (*http.Client, *httptest.Server) {
	smtp.Sender = nil
	a := &API{db: d, emailer: smtp}

	r := mux.NewRouter()
	a.RegisterRoutes(r)
	s := httptest.NewServer(r)

	c := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return c, s
}

func TestAPI_ChangeOrgField(t *testing.T) {
	database := dbtest.Setup(t)
	defer dbtest.Cleanup(t, database)

	cl, server := setup(database)
	defer server.Close()

	user, org := dbtest.GetOrg(t, database)

	var sent bool
	smtp.Sender = func(e *email.Email) error {
		assert.Equal(t, user.Email, e.To[0])
		sent = true
		return nil
	}

	expiresBefore := org.TrialExpiresAt
	res, err := cl.PostForm(
		fmt.Sprintf("%s/admin/users/organizations/%s", server.URL, org.ExternalID),
		url.Values{"field": {"FeatureFlags"}, "value": {"foo billing moo"}},
	)
	assert.NoError(t, err)
	defer res.Body.Close()

	assert.True(t, sent)
	newOrg, _ := database.FindOrganizationByID(ctx, org.ExternalID)
	assert.True(t, expiresBefore.Before(newOrg.TrialExpiresAt))
}

func TestAPI_GetUserToken(t *testing.T) {
	db, _ := memory.New("", "", 1)
	cl, server := setup(db)
	defer server.Close()

	usr, _ := db.CreateUser(nil, "test@test")
	db.AddLoginToUser(nil, usr.ID, "github", "12345", ghSession)

	res, err := cl.Get(fmt.Sprintf("%s/admin/users/users/%v/logins/github/token", server.URL, usr.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expecting StatusOK, got %v", res.StatusCode)
	}
	var tok client.ProviderToken
	err = json.NewDecoder(res.Body).Decode(&tok)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != ghToken {
		t.Fatalf("Expecting db to return token, got %v", tok.Token)
	}
}

func TestAPI_GetUserTokenNoUser(t *testing.T) {
	db, _ := memory.New("", "", 1)
	cl, server := setup(db)
	defer server.Close()

	res, err := cl.Get(fmt.Sprintf("%s/admin/users/users/%v/logins/github/token", server.URL, "unknown"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("Expecting StatusNotFound, got %v", res.StatusCode)
	}
}

func TestAPI_GetUserTokenNoToken(t *testing.T) {
	db, _ := memory.New("", "", 1)
	cl, server := setup(db)
	defer server.Close()

	usr, _ := db.CreateUser(nil, "test@test")

	res, err := cl.Get(fmt.Sprintf("%s/admin/users/users/%v/logins/github/token", server.URL, usr.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expecting StatusUnauthorized, got %v", res.StatusCode)
	}
}

func TestAPI_GetUserTokenInvalidRouteNoUser(t *testing.T) {
	a := API{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.getUserToken(w, r)
	}))
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("Expecting StatusUnprocessableEntity, got %v", res.StatusCode)
	}
}
