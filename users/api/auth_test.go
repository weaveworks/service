package api_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/users/api"
)

func Test_parseAuthHeader(t *testing.T) {
	tests := []struct {
		input  string
		output *api.Credentials
		ok     bool
	}{
		{``, nil, false},
		{`Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==`, &api.Credentials{Realm: "Basic", Params: map[string]string{"basic": "QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}}, true},
		{`Bearer QWxhZGRpbjpvcGVuIHNlc2FtZQ==`, &api.Credentials{Realm: "Bearer", Params: map[string]string{"bearer": "QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}}, true},
		{`Scope-Probe token=oiu38ufoialsmlsi913`, &api.Credentials{Realm: "Scope-Probe", Params: map[string]string{"token": "oiu38ufoialsmlsi913"}}, true},
		{`Digest username=Mufasa,qop=auth`, &api.Credentials{Realm: "Digest", Params: map[string]string{"username": "Mufasa", "qop": "auth"}}, true},
		{`APIKey apiKeyHere`, &api.Credentials{Realm: "APIKey", Params: map[string]string{"apiKeyHere": ""}}, true},
	}
	for _, test := range tests {
		output, ok := api.ParseAuthHeader(test.input)
		failMessage := fmt.Sprintf("%q => %#v, %v", test.input, output, ok)
		if assert.Equal(t, test.ok, ok, failMessage) {
			assert.Equal(t, test.output, output, failMessage)
		}
	}
}
