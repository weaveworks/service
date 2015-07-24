package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	mathRand "math/rand"
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
			o, err := s.createOrganization()
			if err == nil {
				user.OrganizationID = o.ID
				user.OrganizationName = o.Name
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
	user, err := s.FindUserByID(id)
	if err != nil {
		return "", err
	}
	raw, err := secureRandomBase64(128)
	if err != nil {
		return "", err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), passwordHashingCost)
	if err != nil {
		return "", err
	}
	return raw, nil
}

func secureRandomBase64(byteCount int) (string, error) {
	randomData := make([]byte, byteCount)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(randomData), nil
}

func (s memoryStorage) createOrganization() (*Organization, error) {
	o := &Organization{
		ID:   fmt.Sprint(len(s.organizations)),
		Name: fmt.Sprintf("%s-%d", petname.Generate(2, "-"), mathRand.Int31n(100)),
	}

	s.organizations[o.ID] = o
	return o, nil
}
