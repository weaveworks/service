package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/Sirupsen/logrus"
	_ "github.com/mattes/migrate/driver/postgres"
	"github.com/mattes/migrate/migrate"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/weaveworks/scope/common/instrument"
)

var (
	errNotFound       = errors.New("Not found")
	errEmailIsTaken   = validationErrorf("Email is already taken")
	errOrgNameIsTaken = validationErrorf("Organization name is already taken")
)

type database interface {
	// Create a user. The driver should set ID to some default only when it is "".
	CreateUser(email string) (*user, error)

	findUserByIDer
	FindUserByEmail(email string) (*user, error)
	FindUserByLogin(provider, id string) (*user, error)

	// AddLoginToUser adds an entry denoting this user is linked to a
	// remote login. e.g. if a user logs in via github this maps our
	// account to the github account.
	// Note: Must be idempotent!
	AddLoginToUser(userID, provider, id string, session json.RawMessage) error

	// DetachLoginFromUser removes all entries an entry denoting this
	// user is linked to the remote login.
	DetachLoginFromUser(userID, provider string) error

	// Create a new user in an existing organization.
	// If the user already exists:
	// * in a *different* organization, this should return errEmailIsTaken.
	// * but is not approved, approve them into the organization.
	// * in the same organization, no-op.
	InviteUser(email, orgName string) (*user, error)

	// Ensure a user is deleted. If they do not exist, return success.
	DeleteUser(email string) error

	ListUsers(...filter) ([]*user, error)
	ListOrganizationUsers(orgName string) ([]*user, error)

	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*user, error)

	// Set the admin flag of a user
	SetUserAdmin(id string, value bool) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	// Update the user's first login timestamp. Should be called the first time a user logs in (i.e. if FirstLoginAt.IsZero())
	SetUserFirstLoginAt(id string) error

	// Create a new organization owned by the user. The name will be unique and randomly generated.
	CreateOrganization(ownerID string) (*organization, error)
	FindOrganizationByProbeToken(probeToken string) (*organization, error)
	RenameOrganization(oldName, newName string) error

	Close() error
}

type findUserByIDer interface {
	FindUserByID(id string) (*user, error)
}

func mustNewDatabase(databaseURI, migrationsDir string) database {
	u, err := url.Parse(databaseURI)
	if err != nil {
		logrus.Fatal(err)
	}
	if migrationsDir != "" {
		logrus.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				logrus.Error(err)
			}
			logrus.Fatal("Database migrations failed")
		}
	}
	var storage database
	switch u.Scheme {
	case "postgres":
		db, err := sql.Open(u.Scheme, databaseURI)
		if err != nil {
			logrus.Fatal(err)
		}
		storage = newPGStorage(db)
	case "memory":
		storage = newMemoryStorage()
	default:
		logrus.Fatalf("Unknown database type: %s", u.Scheme)
	}
	return timedDatabase{storage, databaseRequestDuration}
}

type timedDatabase struct {
	d        database
	Duration *prometheus.SummaryVec
}

func (t timedDatabase) errorCode(err error) string {
	switch err {
	case nil:
		return "200"
	case errNotFound:
		return "404"
	case errEmailIsTaken:
		return "400"
	case errInvalidAuthenticationData:
		return "401"
	default:
		return "500"
	}
}

func (t timedDatabase) timeRequest(method string, f func() error) error {
	return instrument.TimeRequestStatus(method, t.Duration, t.errorCode, f)
}

func (t timedDatabase) CreateUser(email string) (u *user, err error) {
	t.timeRequest("CreateUser", func() error {
		u, err = t.d.CreateUser(email)
		return err
	})
	return
}

func (t timedDatabase) FindUserByID(id string) (u *user, err error) {
	t.timeRequest("FindUserByID", func() error {
		u, err = t.d.FindUserByID(id)
		return err
	})
	return
}

func (t timedDatabase) FindUserByEmail(email string) (u *user, err error) {
	t.timeRequest("FindUserByEmail", func() error {
		u, err = t.d.FindUserByEmail(email)
		return err
	})
	return
}

func (t timedDatabase) FindUserByLogin(provider, id string) (u *user, err error) {
	t.timeRequest("FindUserByLogin", func() error {
		u, err = t.d.FindUserByLogin(provider, id)
		return err
	})
	return
}

func (t timedDatabase) AddLoginToUser(userID, provider, id string, session json.RawMessage) error {
	return t.timeRequest("AddLoginToUser", func() error {
		return t.d.AddLoginToUser(userID, provider, id, session)
	})
}

func (t timedDatabase) DetachLoginFromUser(userID, provider string) error {
	return t.timeRequest("DetachLoginFromUser", func() error {
		return t.d.DetachLoginFromUser(userID, provider)
	})
}

func (t timedDatabase) InviteUser(email, orgName string) (u *user, err error) {
	t.timeRequest("InviteUser", func() error {
		u, err = t.d.InviteUser(email, orgName)
		return err
	})
	return
}

func (t timedDatabase) DeleteUser(email string) error {
	return t.timeRequest("DeleteUser", func() error {
		return t.d.DeleteUser(email)
	})
}

func (t timedDatabase) ListUsers(fs ...filter) (us []*user, err error) {
	t.timeRequest("ListUsers", func() error {
		us, err = t.d.ListUsers(fs...)
		return err
	})
	return
}

func (t timedDatabase) ListOrganizationUsers(orgName string) (us []*user, err error) {
	t.timeRequest("ListOrganizationUsers", func() error {
		us, err = t.d.ListOrganizationUsers(orgName)
		return err
	})
	return
}

func (t timedDatabase) ApproveUser(id string) (u *user, err error) {
	t.timeRequest("ApproveUser", func() error {
		u, err = t.d.ApproveUser(id)
		return err
	})
	return
}

func (t timedDatabase) SetUserAdmin(id string, value bool) error {
	return t.timeRequest("SetUserAdmin", func() error {
		return t.d.SetUserAdmin(id, value)
	})
}

func (t timedDatabase) SetUserToken(id, token string) error {
	return t.timeRequest("SetUserToken", func() error {
		return t.d.SetUserToken(id, token)
	})
}

func (t timedDatabase) SetUserFirstLoginAt(id string) error {
	return t.timeRequest("SetUserFirstLoginAt", func() error {
		return t.d.SetUserFirstLoginAt(id)
	})
}

func (t timedDatabase) CreateOrganization(ownerID string) (o *organization, err error) {
	t.timeRequest("CreateOrganization", func() error {
		o, err = t.d.CreateOrganization(ownerID)
		return err
	})
	return
}

func (t timedDatabase) FindOrganizationByProbeToken(probeToken string) (o *organization, err error) {
	t.timeRequest("FindOrganizationByProbeToken", func() error {
		o, err = t.d.FindOrganizationByProbeToken(probeToken)
		return err
	})
	return
}

func (t timedDatabase) RenameOrganization(oldName, newName string) error {
	return t.timeRequest("RenameOrganization", func() error {
		return t.d.RenameOrganization(oldName, newName)
	})
}

func (t timedDatabase) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}
