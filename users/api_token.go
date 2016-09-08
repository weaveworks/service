package users

import (
	"time"

	"github.com/weaveworks/service/users/tokens"
)

// APIToken is for use by CLI tools, to authenticate as a user (with access to
// multiple instances).
type APIToken struct {
	ID          string    `json:"-"`
	UserID      string    `json:"-"`
	Token       string    `json:"token"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"-"`
}

// FormatCreatedAt formats a tokens created at timestamp
func (t *APIToken) FormatCreatedAt() string {
	return formatTimestamp(t.CreatedAt)
}

// RegenerateToken regenerates the token.
func (t *APIToken) RegenerateToken() error {
	token, err := tokens.Generate()
	if err != nil {
		return err
	}
	t.Token = token
	return nil
}
