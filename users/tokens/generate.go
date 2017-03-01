package tokens

import (
	"crypto/rand"
	"encoding/base32"
	"net/http"
	"strings"
)

// Exported for testing
const (
	AuthHeaderName = "Authorization"
	Prefix         = "Scope-Probe token="
)

var (
	zbase32 = base32.NewEncoding("ybndrfg8ejkmcpqxot1uwisza345h769")

	// You want charCount to be large enough to provide lots of uniqueness, but
	// small enough to be easy for users. Ideally you want charCount and
	// byteCount to both work out as whole integers, otherwise you will have ====
	// on the end of the token, which is ugly when urlencoded.
	charCount = 32
	byteCount = charCount * 5 / 8 // base32 uses 8 characters per 5 bytes
)

// Generate generates a new cryptographically-secure token (e.g. for probe tokens)
func Generate() (string, error) {
	var (
		randomData = make([]byte, byteCount)
	)
	_, err := rand.Read(randomData)
	if err != nil {
		return "", err
	}
	return zbase32.EncodeToString(randomData), nil
}

// ExtractToken extracts an auth token from a request, if possible.
func ExtractToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get(AuthHeaderName)
	if strings.HasPrefix(authHeader, Prefix) {
		return strings.TrimPrefix(authHeader, Prefix), true
	}

	// To allow grafana to talk to the service, we also accept basic auth,
	// ignoring the username and treating the password as the token.
	_, token, ok := r.BasicAuth()
	return token, ok
}
