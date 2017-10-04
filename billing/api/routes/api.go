package routes

import (
	"flag"
	"html/template"
	"net/http"

	"encoding/base64"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/weaveworks/service/billing/db"
	"github.com/weaveworks/service/billing/zuora"
	"github.com/weaveworks/service/users"
)

// Config holds settings for the API.
type Config struct {
	CORSAllowOrigin string
	AdminURL        string
	HMACSecret      string
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.CORSAllowOrigin, "cors.allow.origin", "https://cloud.weave.works", "Sets the Access-Control-Allow-Origin header")
	f.StringVar(&c.AdminURL, "admin.url", "/admin", "prefix root of link to organization details")
	f.StringVar(&c.HMACSecret, "hmac.secret", "", "Secret for generating HMAC signatures")
}

// API is the billing api
type API struct {
	Config
	DB            db.DB
	Users         users.UsersClient
	Zuora         zuora.Client
	adminTemplate *template.Template
	HMACSecret    []byte
	http.Handler
}

// New creates a new APi
func New(cfg Config, db db.DB, users users.UsersClient, zuora zuora.Client) (*API, error) {
	if cfg.HMACSecret == "" {
		log.Warn("HMAC key is empty")
	}
	hmac, err := base64.StdEncoding.DecodeString(cfg.HMACSecret)
	if err != nil {
		return nil, err
	}

	a := &API{
		Config:        cfg,
		DB:            db,
		Users:         users,
		Zuora:         zuora,
		adminTemplate: template.Must(template.New("admin").Parse(adminTemplate)),
		HMACSecret:    hmac,
	}

	r := mux.NewRouter()
	a.RegisterRoutes(r)
	a.Handler = r
	return a, nil
}
