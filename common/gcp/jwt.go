package gcp

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
)

const (
	expectedIssuer   = "https://www.googleapis.com/robot/v1/metadata/x509/cloud-commerce-partner@system.gserviceaccount.com"
	expectedAudience = "cloud.weave.works"
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
	if !c.VerifyAudience(expectedAudience, true) {
		return fmt.Errorf("unexpected audience: %q", c.Issuer)
	}
	if !c.VerifyIssuer(expectedIssuer, true) {
		return fmt.Errorf("unexpected issuer: %q", c.Issuer)
	}
	if c.Subject == "" {
		return fmt.Errorf("unexpected subject: %q", c.Issuer)
	}
	return nil
}

// ParseJWT reads the JWT received from GCP.
func ParseJWT(tok string) (Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tok, claims, func(token *jwt.Token) (interface{}, error) {
		return token.Header["kid"], nil
	})
	if err != nil {
		return claims, nil
	}
	claims = token.Claims.(Claims)
	return claims, nil
}
