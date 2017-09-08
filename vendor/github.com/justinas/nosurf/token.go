package nosurf

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	tokenLength = 32
)

/*
There are two types of tokens.

* The unmasked "real" token consists of 32 random bytes.
  It is stored in a cookie (base64-encoded) and it's the
  "reference" value that sent tokens get compared to.

* The masked "sent" token consists of 64 bytes:
  32 byte key used for one-time pad masking and
  32 byte "real" token masked with the said key.
  It is used as a value (base64-encoded as well)
  in forms and/or headers.

Upon processing, both tokens are base64-decoded
and then treated as 32/64 byte slices.
*/

// A token is generated by returning tokenLength bytes
// from crypto/rand
func generateToken() []byte {
	bytes := make([]byte, tokenLength)

	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		panic(err)
	}

	return bytes
}

func b64encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func b64decode(data string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil
	}
	return decoded
}

// VerifyToken verifies the sent token equals the real one
// and returns a bool value indicating if tokens are equal.
// Supports masked tokens. realToken comes from Token(r) and
// sentToken is token sent unusual way.
func VerifyToken(realToken, sentToken string) bool {
	r := b64decode(realToken)
	if len(r) == 2*tokenLength {
		r = unmaskToken(r)
	}
	s := b64decode(sentToken)
	if len(s) == 2*tokenLength {
		s = unmaskToken(s)
	}
	return subtle.ConstantTimeCompare(r, s) == 1
}

func verifyToken(realToken, sentToken []byte) bool {
	realN := len(realToken)
	sentN := len(sentToken)

	// sentN == tokenLength means the token is unmasked
	// sentN == 2*tokenLength means the token is masked.

	if realN == tokenLength && sentN == 2*tokenLength {
		return verifyMasked(realToken, sentToken)
	}
	return false
}

// Verifies the masked token
func verifyMasked(realToken, sentToken []byte) bool {
	sentPlain := unmaskToken(sentToken)
	return subtle.ConstantTimeCompare(realToken, sentPlain) == 1
}

func checkForPRNG() {
	// Check that cryptographically secure PRNG is available
	// In case it's not, panic.
	buf := make([]byte, 1)
	_, err := io.ReadFull(rand.Reader, buf)

	if err != nil {
		panic(fmt.Sprintf("crypto/rand is unavailable: Read() failed with %#v", err))
	}
}

func init() {
	checkForPRNG()
}
