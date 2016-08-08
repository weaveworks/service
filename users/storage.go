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
	errForbidden                  = errors.New("Forbidden found")
	errNotFound                   = errors.New("Not found")
	errEmailIsTaken               = validationErrorf("Email is already taken")
	errOrgExternalIDIsTaken       = validationErrorf("ID is already taken")
	errOrgExternalIDCannotBeBlank = validationErrorf("ID cannot be blank")
	errOrgExternalIDFormat        = validationErrorf("ID can only contain letters, numbers, hyphen, and underscore")
	errOrgNameCannotBeBlank       = validationErrorf("Name cannot be blank")
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

	// Invite a user to access an existing organization.
	InviteUser(email, orgExternalID string) (*user, bool, error)

	// Remove a user from an organization. If they do not exist (or are not a member of the org), return success.
	RemoveUserFromOrganization(orgExternalID, email string) error

	ListUsers(...filter) ([]*user, error)
	ListOrganizationUsers(orgExternalID string) ([]*user, error)

	// Approve the user for access. Should generate them a new organization.
	ApproveUser(id string) (*user, error)

	// Set the admin flag of a user
	SetUserAdmin(id string, value bool) error

	// Update the user's login token. Setting the token to "" should disable the
	// user's token.
	SetUserToken(id, token string) error

	// Update the user's first login timestamp. Should be called the first time a user logs in (i.e. if FirstLoginAt.IsZero())
	SetUserFirstLoginAt(id string) error

	// GenerateOrganizationExternalID generates a new, available organization ExternalID
	GenerateOrganizationExternalID() (string, error)

	// Create a new organization owned by the user. ExternalID and name cannot be blank.
	// ExternalID must match the ExternalID regex.
	CreateOrganization(ownerID, externalID, name string) (*organization, error)
	FindOrganizationByProbeToken(probeToken string) (*organization, error)
	RenameOrganization(externalID, newName string) error
	OrganizationExists(externalID string) (bool, error)
	GetOrganizationName(externalID string) (string, error)
	DeleteOrganization(externalID string) error

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
	Duration *prometheus.HistogramVec
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
	return instrument.TimeRequestHistogramStatus(method, t.Duration, t.errorCode, f)
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

func (t timedDatabase) InviteUser(email, orgExternalID string) (u *user, created bool, err error) {
	t.timeRequest("InviteUser", func() error {
		u, created, err = t.d.InviteUser(email, orgExternalID)
		return err
	})
	return
}

func (t timedDatabase) RemoveUserFromOrganization(orgExternalID, email string) error {
	return t.timeRequest("RemoveUserFromOrganization", func() error {
		return t.d.RemoveUserFromOrganization(orgExternalID, email)
	})
}

func (t timedDatabase) ListUsers(fs ...filter) (us []*user, err error) {
	t.timeRequest("ListUsers", func() error {
		us, err = t.d.ListUsers(fs...)
		return err
	})
	return
}

func (t timedDatabase) ListOrganizationUsers(orgExternalID string) (us []*user, err error) {
	t.timeRequest("ListOrganizationUsers", func() error {
		us, err = t.d.ListOrganizationUsers(orgExternalID)
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

func (t timedDatabase) GenerateOrganizationExternalID() (s string, err error) {
	t.timeRequest("GenerateOrganizationExternalID", func() error {
		s, err = t.d.GenerateOrganizationExternalID()
		return err
	})
	return
}

func (t timedDatabase) CreateOrganization(ownerID, externalID, name string) (o *organization, err error) {
	t.timeRequest("CreateOrganization", func() error {
		o, err = t.d.CreateOrganization(ownerID, externalID, name)
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

func (t timedDatabase) RenameOrganization(externalID, name string) error {
	return t.timeRequest("RenameOrganization", func() error {
		return t.d.RenameOrganization(externalID, name)
	})
}

func (t timedDatabase) OrganizationExists(externalID string) (b bool, err error) {
	t.timeRequest("OrganizationExists", func() error {
		b, err = t.d.OrganizationExists(externalID)
		return err
	})
	return
}

func (t timedDatabase) GetOrganizationName(externalID string) (name string, err error) {
	t.timeRequest("GetOrganizationName", func() error {
		name, err = t.d.GetOrganizationName(externalID)
		return err
	})
	return
}

func (t timedDatabase) DeleteOrganization(externalID string) error {
	return t.timeRequest("DeleteOrganization", func() error {
		return t.d.DeleteOrganization(externalID)
	})
}

func (t timedDatabase) Close() error {
	return t.timeRequest("Close", func() error {
		return t.d.Close()
	})
}
