package attrsync

import (
	"github.com/FrenchBen/goketo"

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
