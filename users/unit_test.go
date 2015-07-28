// +build !integration

package main

import (
	"fmt"
	mathRand "math/rand"
	"testing"
	"time"

	"github.com/dustinkirkland/golang-petname"
	"github.com/jordan-wright/email"
	"golang.org/x/crypto/bcrypt"
)

var sentEmails []*email.Email

func setup(t *testing.T) {
	domain = "example.com"
	passwordHashingCost = bcrypt.MinCost
	sentEmails = nil
	sendEmail = testEmailSender

	setupLogging("debug")
	storage = &memoryStorage{
		users:         make(map[string]*User),
		organizations: make(map[string]*Organization),
	}
	setupTemplates()
	setupSessions("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd")
}

func cleanup(t *testing.T) {
}

func testEmailSender(e *email.Email) error {
	sentEmails = append(sentEmails, e)
	return nil
}

type memoryStorage struct {
	users         map[string]*User
	organizations map[string]*Organization
}

func (s memoryStorage) CreateUser(email string) (*User, error) {
	u := &User{
		ID:    fmt.Sprint(len(s.users)),
		Email: email,
	}
	s.users[u.ID] = u
	return u, nil
}

func (s memoryStorage) FindUserByID(id string) (*User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s memoryStorage) FindUserByEmail(email string) (*User, error) {
	for _, user := range s.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, ErrNotFound
}

func (s memoryStorage) ApproveUser(id string) error {
	user, err := s.FindUserByID(id)
	if err != nil {
		return err
	}

	o, err := s.createOrganization()
	if err == nil {
		user.ApprovedAt = time.Now().UTC()
		user.OrganizationID = o.ID
		user.OrganizationName = o.Name
	}
	return err
}

func (s memoryStorage) SetUserToken(id, token string) error {
	var hashed []byte
	if token != "" {
		var err error
		hashed, err = bcrypt.GenerateFromPassword([]byte(token), passwordHashingCost)
		if err != nil {
			return err
		}
	}
	user, ok := s.users[id]
	if !ok {
		return ErrNotFound
	}
	user.Token = string(hashed)
	user.TokenCreatedAt = time.Now().UTC()
	return nil
}

func (s memoryStorage) createOrganization() (*Organization, error) {
	o := &Organization{
		ID:   fmt.Sprint(len(s.organizations)),
		Name: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), mathRand.Int31n(100)),
	}
	s.organizations[o.ID] = o
	return o, nil
}

func (s memoryStorage) Close() error {
	return nil
}
