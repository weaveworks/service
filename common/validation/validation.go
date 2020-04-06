package validation

import (
	"regexp"
	"strings"
)

// ValidateEmail validates for format of an email.
func ValidateEmail(email string) bool {
	if len(email) > 254 { //tools.ietf.org/html/rfc5321#section-4.5.3
		return false
	}
	// https://www.regular-expressions.info/email.html explains the following regexp matches 99.99% email addresses in use.
	re := regexp.MustCompile("\\A[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\z")
	return re.MatchString(strings.ToLower(email))
}

// ValidateName validates a first or second name
func ValidateName(name string) bool {
	return len(name) <= 100
}
