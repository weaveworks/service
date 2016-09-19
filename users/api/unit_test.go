// +build !integration

package api_test

import (
	"testing"

	"github.com/jordan-wright/email"
	"golang.org/x/crypto/bcrypt"

	"github.com/weaveworks/service/common/logging"
	"github.com/weaveworks/service/users/api"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/emailer"
	"github.com/weaveworks/service/users/login"
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
	db.PasswordHashingCost = bcrypt.MinCost

	var directLogin = false

	logging.Setup("debug")
	database = dbtest.Setup(t)
	sessionStore = sessions.MustNewStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd")
	templates := templates.MustNewEngine("../templates")
	logins = login.NewProviders()

	sentEmails = nil
	emailer := emailer.SMTPEmailer{
		Templates:   templates,
		Sender:      testEmailSender,
		Domain:      domain,
		FromAddress: "test@test.com",
	}
	app = api.New(directLogin, emailer, sessionStore, database, logins, templates, nil, nil)
}

func cleanup(t *testing.T) {
	logins.Reset()
	dbtest.Cleanup(t, database)
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}
