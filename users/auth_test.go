package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseAuthHeader(t *testing.T) {
	tests := []struct {
		input  string
		output *credentials
		ok     bool
	}{
		{``, nil, false},
		{`Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==`, &credentials{Realm: "Basic", Params: map[string]string{"basic": "QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}}, true},
		{`Bearer QWxhZGRpbjpvcGVuIHNlc2FtZQ==`, &credentials{Realm: "Bearer", Params: map[string]string{"bearer": "QWxhZGRpbjpvcGVuIHNlc2FtZQ=="}}, true},
		{`Scope-Probe token=oiu38ufoialsmlsi913`, &credentials{Realm: "Scope-Probe", Params: map[string]string{"token": "oiu38ufoialsmlsi913"}}, true},
		{`Digest username=Mufasa,qop=auth`, &credentials{Realm: "Digest", Params: map[string]string{"username": "Mufasa", "qop": "auth"}}, true},
	}
	for _, test := range tests {
		output, ok := parseAuthHeader(test.input)
		failMessage := fmt.Sprintf("%q => %#v, %v", test.input, output, ok)
		if assert.Equal(t, test.ok, ok, failMessage) {
			assert.Equal(t, test.output, output, failMessage)
		}
	}
}
