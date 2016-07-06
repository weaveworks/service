package common

import (
	"database/sql"
)

// States the deployments can be in
const (
	Pending = "pending"
	Running = "running"
	Success = "success"
	Failed  = "failed"
	Skipped = "skipped"
)

// Deployment describes a deployment
type Deployment struct {
	ID        string `json:"id"`
	ImageName string `json:"image_name"`
	Version   string `json:"version"`
	Priority  int    `json:"priority"`
	State     string `json:"status"`
	LogKey    string `json:"-"`
}

// DeployStore stores info about deployments.
type DeployStore struct {
	db *sql.DB
}

// NewDeployStore creates a new DeployStore
func NewDeployStore(db *sql.DB) *DeployStore {
	return &DeployStore{
		db: db,
	}
}

func (d *DeployStore) tx(f func(*sql.Tx) error) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if err := f(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// StoreNewDeployment stores a new deployment in the pending state.
func (d *DeployStore) StoreNewDeployment(orgID string, deployment Deployment) error {
	return d.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`UPDATE deploys
			    SET state = $1
			  WHERE state = $2
			    AND organization_id = $3
			    AND image = $4`,
			Skipped,
			Pending,
			orgID,
			deployment.ImageName,
		); err != nil {
			return err
		}

		_, err := tx.Exec(
			`INSERT INTO deploys (organization_id, image, version, priority, state)
			 VALUES ($1, $2, $3, $4, $5)`,
			orgID,
			deployment.ImageName,
			deployment.Version,
			deployment.Priority,
			Pending,
		)
		return err
	})
}

// UpdateDeploymentState updates the state of a deployment
func (d *DeployStore) UpdateDeploymentState(id, state, logKey string) error {
	_, err := d.db.Exec(
		`UPDATE deploys
		    SET (state, log_key) = ($1, $2)
		  WHERE id = $3`,
		state,
		logKey,
		id,
	)
	return err
}

// GetDeployments fetches deployments from the database
func (d *DeployStore) GetDeployments(orgID string) ([]Deployment, error) {
	result := []Deployment{}
	rows, err := d.db.Query(
		`SELECT id, image, version, priority, state, log_key
		   FROM deploys
		  WHERE organization_id = $1
	   ORDER BY id::Integer DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var deployment Deployment
		var logKey sql.NullString
		if err := rows.Scan(
			&deployment.ID, &deployment.ImageName, &deployment.Version,
			&deployment.Priority, &deployment.State, &logKey,
		); err != nil {
			return nil, err
		}
		deployment.LogKey = logKey.String
		result = append(result, deployment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDeployment fetches a deployment from the database
func (d *DeployStore) GetDeployment(orgID, deployID string) (*Deployment, error) {
	var deployment Deployment
	var logKey sql.NullString
	if err := d.db.QueryRow(
		`SELECT id, image, version, priority, state, log_key
		   FROM deploys
		  WHERE organization_id = $1
		    AND id = $2`,
		orgID, deployID,
	).Scan(
		&deployment.ID, &deployment.ImageName, &deployment.Version,
		&deployment.Priority, &deployment.State, &logKey,
	); err == sql.ErrNoRows {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	deployment.LogKey = logKey.String
	return &deployment, nil
}

// GetNextDeployment fetches the next deployment to "do", and marks it as running.
func (d *DeployStore) GetNextDeployment() (string, *Deployment, error) {
	// Query the database for the lowest, "new" deploy, claim it,
	// find any subsequent deploys for the same image, pick the latest
	// and make the rest of the deploys for the same image as "skipped"
	var (
		orgID      string
		deployment Deployment
	)
	err := d.tx(func(tx *sql.Tx) error {
		// Query the database for the lowest pending deploy
		if err := tx.QueryRow(`
			SELECT id, organization_id, image, version
			  FROM deploys
			 WHERE state = $1
		  ORDER BY id::Integer DESC
			 LIMIT 1`,
			Pending,
		).Scan(&deployment.ID, &orgID, &deployment.ImageName, &deployment.Version); err != nil {
			return err
		}
		// And mark it as running
		_, err := tx.Exec(`
			UPDATE deploys
			   SET state = $1
			 WHERE id = $2`,
			Running,
			deployment.ID,
		)
		return err
	})
	if err != nil {
		return "", nil, err
	}
	return orgID, &deployment, nil
}
