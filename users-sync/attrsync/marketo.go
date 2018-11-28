package attrsync

import (
	"github.com/FrenchBen/goketo"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/marketing"
)

// NewMarketoClient returns a new marketo client from the connection, auth & program details
// NB: if cfg.ClientID == "" it will return a Noop client
func NewMarketoClient(cfg marketing.MarketoConfig) (marketing.MarketoClient, error) {
	if cfg.ClientID == "" {
		return &marketing.NoopMarketoClient{}, nil
	}

	goketoClient, err := goketo.NewAuthClient(
		cfg.ClientID, cfg.Secret, cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	return marketing.NewMarketoClient(goketoClient, cfg.Program), nil
}

func marketoProspect(user *users.User) (prospect marketing.Prospect, ok bool) {
	prospect.Email = user.Email

	if user.FirstName != "" {
		prospect.FirstName = user.FirstName
		ok = true
	}
	if user.LastName != "" {
		prospect.LastName = user.LastName
		ok = true
	}
	if user.Company != "" {
		prospect.Company = user.Company
		ok = true
	}

	return prospect, ok
}
