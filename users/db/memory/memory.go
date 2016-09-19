package memory

import (
	"sync"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/login"
)

func init() {
	db.Register("memory", New)
}

type memoryDB struct {
	users         map[string]*users.User
	organizations map[string]*users.Organization
	memberships   map[string][]string
	logins        map[string]*login.Login
	apiTokens     map[string]*users.APIToken
	mtx           sync.Mutex
}

// New creates a new in-memory database
func New(_, _ string) (db.DB, error) {
	return &memoryDB{
		users:         make(map[string]*users.User),
		organizations: make(map[string]*users.Organization),
		memberships:   make(map[string][]string),
		logins:        make(map[string]*login.Login),
		apiTokens:     make(map[string]*users.APIToken),
	}, nil
}

func (s *memoryDB) Close() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return nil
}
