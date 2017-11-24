package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/validation"
)

func TestValidateEmail(t *testing.T) {
	for input, expected := range map[string]bool{
		"email@domain.com":              true,
		"a@domain.com":                  true,
		"aa@domain.com":                 true,
		"AA@domain.com":                 true,
		"Aa@domain.com":                 true,
		"AA@DOMAIN.COM":                 true,
		"abc@domain.com":                true,
		"email-email@domain.com":        true,
		"firstname.lastname@domain.com": true,
		"email@subdomain.domain.com":    true,
		"firstname+lastname@domain.com": true,
		"email@123.123.123.123":         true,
		"1234567890@domain.com":         true,
		"email@domain-one.com":          true,
		"_______@domain.com":            true,
		"email@domain.co.uk":            true,
		"firstname-lastname@domain.com": true,
		"user@weave.works":              true,
		"user+abc@weave.works":          true,
		"trailingspace@domain.com ":     false,
		"trailingspace2@domain.com  ":   false,
		" leadingspace@domain.com":      false,
		" leadingspace2@domain.com":     false,
		" spaces@domain.com ":           false,
		"   spaces2@domain.com   ":      false,
		"string":                        false,
		"":                              false,
		"123":                           false,
		"#@%^%#$@#$@#.com":              false,
		"@domain.com":                   false,
		"email.domain.com":              false,
		"mail@domain@domain.com":        false,
		".email@domain.com":             false,
		"email.@domain.com":             false,
		"email@-domain.com":             false,
		"<h1>test</h1>test@yahoo.com":   false,
	} {
		assert.Equal(t, expected, validation.ValidateEmail(input), input)
	}
}
