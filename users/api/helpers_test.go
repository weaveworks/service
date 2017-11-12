package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jordan-wright/email"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/gcp/partner"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/grpc"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/marketing"
	"github.com/weaveworks/service/users/sessions"
	"github.com/weaveworks/service/users/templates"
)

var (
	sentEmails   []*email.Email
	app          *api.API
	database     db.DB
	logins       *login.Providers
	sessionStore sessions.Store
	domain       = "http://fake.scope"
)

func setup(t *testing.T) {
	setupWithMockServices(t, "", "", "", "")
}

func setupWithMockServices(t *testing.T, fluxAPI, scopeAPI, promAPI, netAPI string) {
	db.PasswordHashingCost = bcrypt.MinCost

	var directLogin = false

	database = dbtest.Setup(t)
	sessionStore = sessions.MustNewStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", false)
	templates := templates.MustNewEngine("../templates")
	logins = login.NewProviders()
	mixpanelClient := marketing.NewMixpanelClient("")
	var partnerClient partner.API

	sentEmails = nil
	emailer := emailer.SMTPEmailer{
		Templates:   templates,
		Sender:      testEmailSender,
		Domain:      domain,
		FromAddress: "test@test.com",
	}
	grpcServer := grpc.New(sessionStore, database, nil)
	var billingEnabler featureflag.Enabler
	billingEnabler = featureflag.NewRandomEnabler(0) // Always disabled, does not really matter here.
	app = api.New(
		directLogin,
		emailer,
		sessionStore,
		database,
		logins,
		templates,
		nil,
		nil,
		"",
		"",
		grpcServer,
		make(map[string]struct{}),
		mixpanelClient,
		partnerClient,
		fluxAPI,
		scopeAPI,
		promAPI,
		netAPI,
		billingEnabler,
	)
}

func cleanup(t *testing.T) {
	logins.Reset()
	dbtest.Cleanup(t, database)
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}

// RequestAs makes a request as the given user.
func requestAs(t *testing.T, u *users.User, method, endpoint string, body io.Reader) *http.Request {
	impersonatingUserID := "" // this test doesn't involve impersonation
	cookie, err := sessionStore.Cookie(u.ID, impersonatingUserID)
	assert.NoError(t, err)

	r, err := http.NewRequest(method, endpoint, body)
	require.NoError(t, err)

	r.AddCookie(cookie)
	return r
}

func getUser(t *testing.T) *users.User {
	return dbtest.GetUser(t, database)
}

func createOrgForUser(t *testing.T, u *users.User) *users.Organization {
	return dbtest.CreateOrgForUser(t, database, u)
}

func getOrg(t *testing.T) (*users.User, *users.Organization) {
	return dbtest.GetOrg(t, database)
}

type jsonBody map[string]interface{}

func (j jsonBody) Reader(t *testing.T) io.Reader {
	b, err := json.Marshal(j)
	require.NoError(t, err)
	return bytes.NewReader(b)
}
