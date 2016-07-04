package common

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Config for the deployment system for a user.
type Config struct {
	RepoURL        string `json:"repo_url" yaml:"repo_url"`
	RepoPath       string `json:"repo_path" yaml:"repo_path"`
	RepoKey        string `json:"repo_key" yaml:"repo_key"`
	KubeconfigPath string `json:"kubeconfig_path" yaml:"kubeconfig_path"`
}

// ErrNoConfig is the error returns by GetConfig if the user hasn't set a config yet.
var ErrNoConfig = fmt.Errorf("No config for organisation")

// GetConfig fetches config from the database for a given user.
func GetConfig(db *sql.DB, orgID string) (*Config, error) {
	var confJSON string
	if err := db.QueryRow(
		`SELECT conf
		   FROM conf
		  WHERE organization_id = $1
	   ORDER BY id::Integer DESC
		  LIMIT 1`,
		orgID,
	).Scan(&confJSON); err == sql.ErrNoRows {
		return nil, ErrNoConfig
	} else if err != nil {
		return nil, err
	}

	var conf Config
	if err := json.NewDecoder(strings.NewReader(confJSON)).Decode(&conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

// StoreConfig saves config to the database for a given user.
func StoreConfig(db *sql.DB, orgID string, c Config) error {
	// And reencode...
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&c); err != nil {
		return err
	}

	_, err := db.Exec(
		"INSERT INTO conf (organization_id, conf) VALUES ($1, $2)",
		orgID,
		buf.String(),
	)
	return err
}
