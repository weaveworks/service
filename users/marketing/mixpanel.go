package marketing

import (
	"github.com/dukex/mixpanel"
)

const (
	loginEventName  = "backend.user.login"
	signupEventName = "backend.user.signup"
)

// MixpanelClient wraps the mixpanel library
type MixpanelClient struct {
	client mixpanel.Mixpanel
}

// NewMixpanelClient returns a new MixpanelClient
func NewMixpanelClient(token string) *MixpanelClient {
	return &MixpanelClient{
		// Second arg is mixpanel api URL. Empty string goes to default api host.
		client: mixpanel.New(token, ""),
	}
}

// TrackLogin sends a login event to mixpanel
func (m *MixpanelClient) TrackLogin(email string, firstLogin bool) error {
	return m.client.Track(email, loginEventName, &mixpanel.Event{
		Properties: map[string]interface{}{
			"email":      email,
			"firstLogin": firstLogin,
		},
	})
}

// TrackSignup sends a signup event to mixpanel
func (m *MixpanelClient) TrackSignup(email string) error {
	return m.client.Track(email, signupEventName, &mixpanel.Event{
		Properties: map[string]interface{}{
			"email": email,
		},
	})
}
