package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/login"
)

// CreateUser creates a new user with the given email.
func (d *DB) CreateUser(_ context.Context, email string) (*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.createUser(email)
}

func (d *DB) createUser(email string) (*users.User, error) {
	u := &users.User{
		ID:        fmt.Sprint(len(d.users)),
		Email:     strings.ToLower(email),
		CreatedAt: time.Now().UTC(),
	}
	d.users[u.ID] = u
	return u, nil
}

// AddLoginToUser adds the given login to the specified user. If it is already
// attached elsewhere, this will error.
func (d *DB) AddLoginToUser(_ context.Context, userID, provider, providerID string, session json.RawMessage) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	if _, err := d.findUserByID(userID); err != nil {
		return err
	}

	// Check if this login is attached to another user
	existing, err := d.findUserByLogin(provider, providerID)
	if err == nil && existing.ID != userID {
		return users.AlreadyAttachedError{ID: existing.ID, Email: existing.Email}
	}

	// Add it to this one (updating session if needed).
	d.logins[fmt.Sprintf("%s-%s", provider, providerID)] = &login.Login{
		UserID:     userID,
		Provider:   provider,
		ProviderID: providerID,
		Session:    session,
		CreatedAt:  time.Now().UTC(),
	}
	return nil
}

// DetachLoginFromUser detaches the specified login from a user. e.g. if you
// want to attach it to a different user, do this first.
func (d *DB) DetachLoginFromUser(_ context.Context, userID, provider string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	_, err := d.findUserByID(userID)
	if err == users.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	newLogins := make(map[string]*login.Login)
	for k, v := range d.logins {
		if v.UserID != userID || v.Provider != provider {
			newLogins[k] = v
		}
	}
	d.logins = newLogins
	return nil
}

// InviteUser invites the user, to join the organization. If they are already a
// member this is a noop.
func (d *DB) InviteUser(_ context.Context, email, orgExternalID string) (*users.User, bool, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	created := false
	o, err := d.findOrganizationByExternalID(orgExternalID)
	if err != nil {
		return nil, false, err
	}

	u, err := d.findUserByEmail(email)
	if err == users.ErrNotFound {
		u, err = d.createUser(email)
		created = true
	}
	if err != nil {
		return nil, false, err
	}

	isMember, err := d.userIsMemberOf(u.ID, orgExternalID)
	if err != nil {
		return nil, false, err
	}
	if isMember {
		return u, false, nil
	}
	d.memberships[o.ID] = append(d.memberships[o.ID], u.ID)
	return u, created, nil
}

// FindUserByID finds the user by id
func (d *DB) FindUserByID(_ context.Context, id string) (*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.findUserByID(id)
}

func (d *DB) findUserByID(id string) (*users.User, error) {
	u, ok := d.users[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

// FindUserByEmail finds the user by email
func (d *DB) FindUserByEmail(_ context.Context, email string) (*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.findUserByEmail(email)
}

func (d *DB) findUserByEmail(email string) (*users.User, error) {
	for _, user := range d.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, users.ErrNotFound
}

// FindUserByLogin finds the user by login
func (d *DB) FindUserByLogin(_ context.Context, provider, providerID string) (*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	return d.findUserByLogin(provider, providerID)
}

func (d *DB) findUserByLogin(provider, providerID string) (*users.User, error) {
	for _, l := range d.logins {
		if l.Provider == provider && l.ProviderID == providerID {
			return d.findUserByID(l.UserID)
		}
	}
	return nil, users.ErrNotFound
}

type usersByCreatedAt []*users.User

func (u usersByCreatedAt) Len() int           { return len(u) }
func (u usersByCreatedAt) Swap(i, j int)      { u[i], u[j] = u[j], u[i] }
func (u usersByCreatedAt) Less(i, j int) bool { return u[i].CreatedAt.After(u[j].CreatedAt) }

// ListUsers lists users
func (d *DB) ListUsers(_ context.Context, f filter.User) ([]*users.User, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	users := []*users.User{}
	for _, user := range d.users {
		if strings.Contains(user.Email, f.Search) && (!f.Admin || user.Admin) {
			users = append(users, user)
		}
	}
	sort.Sort(usersByCreatedAt(users))
	return users, nil
}

// ListLoginsForUserIDs lists the logins for these users
func (d *DB) ListLoginsForUserIDs(_ context.Context, userIDs ...string) ([]*login.Login, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var logins []*login.Login
	for _, l := range d.logins {
		for _, userID := range userIDs {
			if l.UserID == userID {
				logins = append(logins, l)
				break
			}
		}
	}
	return logins, nil
}

// SetUserAdmin sets the admin flag of a user
func (d *DB) SetUserAdmin(_ context.Context, id string, value bool) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	user, ok := d.users[id]
	if !ok {
		return users.ErrNotFound
	}
	user.Admin = value
	return nil
}

// SetUserToken updates the user's login token
func (d *DB) SetUserToken(_ context.Context, id, token string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), d.passwordHashingCost)
		if err != nil {
			return err
		}
	}
	user, ok := d.users[id]
	if !ok {
		return users.ErrNotFound
	}
	user.Token = string(hashed)
	user.TokenCreatedAt = time.Now().UTC()
	return nil
}

// SetUserFirstLoginAt is called the first time a user logs in, to set their
// first_login_at field.
func (d *DB) SetUserFirstLoginAt(_ context.Context, id string) error {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	user, ok := d.users[id]
	if !ok {
		return users.ErrNotFound
	}
	if user.FirstLoginAt.IsZero() {
		user.FirstLoginAt = time.Now().UTC()
	}
	return nil
}
