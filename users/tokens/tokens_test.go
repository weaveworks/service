package tokens

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ExtractToken(t *testing.T) {
	tests := []struct {
		input string
		token string
		ok    bool
	}{
		{``, "", false},
		{`Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==`, "open sesame", true},
		{`Scope-Probe token=oiu38ufoialsmlsi913`, "oiu38ufoialsmlsi913", true},
		{`Digest username=Mufasa,qop=auth`, "", false},
		{`APIKey apiKeyHere`, "", false},
	}
	for _, test := range tests {
		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", test.input)
		token, ok := ExtractToken(req)
		failMessage := fmt.Sprintf("%q => %#v, %v", test.input, token, ok)
		if assert.Equal(t, test.ok, ok, failMessage) {
			assert.Equal(t, test.token, token, failMessage)
		}
	}
}
