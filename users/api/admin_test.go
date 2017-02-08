package api

import (
	"encoding/json"
	"fmt"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/memory"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	ghToken   = "e12eb509a297f56dcc77c86ec9e44369080698a6"
	ghSession = []byte(`{"token": {"expiry": "0001-01-01T00:00:00Z", "token_type": "bearer", "access_token": "` + ghToken + `"}}`)
)

func TestAdmin_GetUserToken(t *testing.T) {
	db, _ := memory.New("", "", 1)
	usr, _ := db.CreateUser(nil, "test@test")
	db.AddLoginToUser(nil, usr.ID, "github", "12345", ghSession)
	a := API{
		db: db,
	}

	ts := httptest.NewServer(a.routes())
	defer ts.Close()

	res, err := http.Get(fmt.Sprintf("%s/private/api/users/%v/logins/github/token", ts.URL, usr.ID))
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
	a := API{
		db: db,
	}

	ts := httptest.NewServer(a.routes())
	defer ts.Close()

	res, err := http.Get(fmt.Sprintf("%s/private/api/users/%v/logins/github/token", ts.URL, "unknown"))
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
	usr, _ := db.CreateUser(nil, "test@test")
	a := API{
		db: db,
	}

	ts := httptest.NewServer(a.routes())
	defer ts.Close()

	res, err := http.Get(fmt.Sprintf("%s/private/api/users/%v/logins/github/token", ts.URL, usr.ID))
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
