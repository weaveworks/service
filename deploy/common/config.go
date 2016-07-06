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

// ErrNotFound is the error returned when something is not found.
var ErrNotFound = fmt.Errorf("Not found")

// GetConfig fetches config from the database for a given user.
func (d *DeployStore) GetConfig(orgID string) (*Config, error) {
	var confJSON string
	if err := d.db.QueryRow(
		`SELECT conf
		   FROM conf
		  WHERE organization_id = $1
	   ORDER BY id::Integer DESC
		  LIMIT 1`,
		orgID,
	).Scan(&confJSON); err == sql.ErrNoRows {
		return nil, ErrNotFound
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
func (d *DeployStore) StoreConfig(orgID string, c Config) error {
	// And reencode...
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&c); err != nil {
		return err
	}

	_, err := d.db.Exec(
		"INSERT INTO conf (organization_id, conf) VALUES ($1, $2)",
		orgID,
		buf.String(),
	)
	return err
}
