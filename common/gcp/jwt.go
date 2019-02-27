package gcp

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
)

const (
	issuer       = "https://www.googleapis.com/robot/v1/metadata/x509/cloud-commerce-partner@system.gserviceaccount.com"
	audienceDev  = "frontend.dev.weave.works"
	audienceProd = "cloud.weave.works"
)

// Claims implements further verifications for a GCP JWT.
type Claims struct {
	jwt.StandardClaims
}

// Valid verifies the token.
func (c Claims) Valid() error {
	// StandardClaims verifies:
	// - jwt signature is using public key from Google
	// - jwt is valid to use now (time sensitive claims)
	if err := c.StandardClaims.Valid(); err != nil {
		return err
	}
	if !c.VerifyAudience(audienceDev, true) && !c.VerifyAudience(audienceProd, true) {
		return fmt.Errorf("unexpected audience: %q", c.Audience)
	}
	if !c.VerifyIssuer(issuer, true) {
		return fmt.Errorf("unexpected issuer: %q", c.Issuer)
	}
	if c.Subject == "" {
		return fmt.Errorf("unexpected subject: %q", c.Subject)
	}
	return nil
}

// ParseJWT reads and validates the JWT received from GCP.
func ParseJWT(tok string) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tok, &claims, func(token *jwt.Token) (interface{}, error) {
		// Verify this explicitly, see https://auth0.com/blog/critical-vulnerabilities-in-json-web-token-libraries/
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return fetchPublicKey(token.Header["kid"].(string))
	})
	if err != nil {
		return claims, err
	}
	claims = *token.Claims.(*Claims)
	return claims, nil
}

func fetchPublicKey(keyID string) (*rsa.PublicKey, error) {
	resp, err := http.Get(issuer)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var pems map[string]string
	err = json.Unmarshal(body, &pems)
	if err != nil {
		return nil, err
	}
	if _, ok := pems[keyID]; !ok {
		return nil, fmt.Errorf("public key %q not found", keyID)
	}

	pub, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pems[keyID]))
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing public key %q", keyID)
	}
	return pub, nil
}
