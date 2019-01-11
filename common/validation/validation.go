package validation

import (
	"regexp"
	"strings"
)

// ValidateEmail validates for format of an email.
func ValidateEmail(email string) bool {
	// https://www.regular-expressions.info/email.html explains the following regexp matches 99.99% email addresses in use.
	re := regexp.MustCompile("\\A[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\\z")
	return re.MatchString(strings.ToLower(email))
}
