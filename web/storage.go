package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/dustinkirkland/golang-petname"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound = errors.New("Not found")
)

type Storage interface {
	CreateUser(email string) (*User, error)
	FindUserByID(id string) (*User, error)
	FindUserByEmail(email string) (*User, error)
	ApproveUser(id string) error
	ResetUserToken(id string) error
	GenerateUserToken(id string) (string, error)
}

func setupStorage() {
	storage = &memoryStorage{
		users:         make(map[string]*User),
		organizations: make(map[string]*Organization),
	}
}

type memoryStorage struct {
	users         map[string]*User
	organizations map[string]*Organization
}

func (s memoryStorage) CreateUser(email string) (*User, error) {
	s.users[email] = &User{
		ID:    fmt.Sprint(len(s.users)),
		Email: email,
	}
	return s.users[email], nil
}

func (s memoryStorage) FindUserByEmail(email string) (*User, error) {
	s.users[email] = &User{
		Email: email,
	}
	return s.users[email], nil
}

func (s memoryStorage) ApproveUser(id string) error {
	for _, user := range s.users {
		if user.ID == id {
			_, err := s.createOrganization(id)
			if err == nil {
				user.ApprovedAt = time.Now().UTC()
			}
			return err
		}
	}
	return ErrNotFound
}

func (s memoryStorage) ResetUserToken(id string) error {
	for _, user := range s.users {
		if user.ID == id {
			user.Token = ""
			return nil
		}
	}
	return ErrNotFound
}

func (s memoryStorage) GenerateUserToken(id string) (string, error) {
	for _, user := range s.users {
		if user.ID == id {
			raw := randomString()
			hashed, err := bcrypt.GenerateFromPassword([]byte(raw), passwordHashingCost)
			if err != nil {
				return "", err
			}
			user.Token = string(hashed)
			user.TokenExpiry = time.Now().UTC().Add(6 * time.Hour)
			return raw, nil
		}
	}
	return "", ErrNotFound
}

func (s memoryStorage) createOrganization(memberIDs ...string) (*Organization, error) {
	o := &Organization{
		ID:   fmt.Sprint(len(s.organizations)),
		Name: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), rand.Int31n(100)),
	}
	for _, user := range s.users {
		for _, id := range memberIDs {
			if user.ID == id {
				user.OrganizationID = o.ID
				user.OrganizationName = o.Name
			}
		}
	}

	s.organizations[o.ID] = o
	return o, nil
}
