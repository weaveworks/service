package pardot

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/jehiah/go-strftime"
)

const (
	email     = "foo@bar.com"
	password  = "password"
	userkey   = "apikey"
	userEmail = "baz@bar.com"
)

type pardotProspect struct {
	ServiceCreatedAt  string
	ServiceApprovedAt string
	ServiceLastAccess string
}

func TestPardotClient(t *testing.T) {
	prospects := make(chan map[string]pardotProspect)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println("Error reading request body:", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		switch r.URL.Path {
		case loginPath:
			values, err := url.ParseQuery(string(body))
			if err != nil {
				fmt.Println("Error parsing request body:", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if values.Get("email") != email ||
				values.Get("password") != password ||
				values.Get("user_key") != userkey {
				t.Fatal(values)
			}

			if err := xml.NewEncoder(w).Encode(apiResponse{
				APIKey: "foo",
			}); err != nil {
				fmt.Println("Error writing response body:", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

		case batchUpsertPath:
			values, err := url.ParseQuery(string(body))
			if err != nil {
				fmt.Println("Error parsing request body:", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			bodyJSON := values.Get("prospects")
			body := struct {
				Prospects map[string]pardotProspect `json:"prospects"`
			}{}

			if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
				fmt.Println("Error parsing request body:", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			prospects <- body.Prospects

			if err := xml.NewEncoder(w).Encode(apiResponse{}); err != nil {
				fmt.Println("Error writing response body:", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return

		default:
			fmt.Println("Path not recognised:", r.URL.Path)
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := NewClient(ts.URL, email, password, userkey)
	defer client.Stop()

	createdAt := time.Now()
	approvedAt := createdAt.Add(48 * time.Hour)
	client.UserCreated(userEmail, createdAt)
	client.UserApproved(userEmail, approvedAt)
	select {
	case ps := <-prospects:
		if !reflect.DeepEqual(ps, map[string]pardotProspect{
			userEmail: {
				ServiceCreatedAt:  strftime.Format("%Y-%m-%d", createdAt),
				ServiceApprovedAt: strftime.Format("%Y-%m-%d", approvedAt),
			},
		}) {
			t.Fatal("Wrong data: ", ps)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("No prospects recieved.")
	}
}
