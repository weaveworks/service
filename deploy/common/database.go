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
}

func tx(db *sql.DB, f func(*sql.Tx) error) error {
	tx, err := db.Begin()
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
func StoreNewDeployment(db *sql.DB, orgID string, d Deployment) error {
	return tx(db, func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`UPDATE deploys
			    SET state = $1
			  WHERE state = $2
			    AND organization_id = $3
			    AND image = $4`,
			Skipped,
			Pending,
			orgID,
			d.ImageName,
		); err != nil {
			return err
		}

		_, err := tx.Exec(
			`INSERT INTO deploys (organization_id, image, version, priority, state)
			 VALUES ($1, $2, $3, $4, $5)`,
			orgID,
			d.ImageName,
			d.Version,
			d.Priority,
			Pending,
		)
		return err
	})
}

// UpdateDeploymentState updates the state of a deployment
func UpdateDeploymentState(db *sql.DB, id, state string) error {
	_, err := db.Exec(
		`UPDATE deploys
		    SET state = $1
		  WHERE id = $2`,
		state,
		id,
	)
	return err
}

// GetDeployments fetches deployments from the database
func GetDeployments(db *sql.DB, orgID string) ([]Deployment, error) {
	result := []Deployment{}
	rows, err := db.Query(
		`SELECT id, image, version, priority, state
		   FROM deploys
		  WHERE organization_id = $1
	   ORDER BY id::Integer DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var deployment Deployment
		if err := rows.Scan(&deployment.ID, &deployment.ImageName, &deployment.Version, &deployment.Priority, &deployment.State); err != nil {
			return nil, err
		}
		result = append(result, deployment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetNextDeployment fetches the next deployment to "do", and marks it as running.
func GetNextDeployment(db *sql.DB) (string, *Deployment, error) {
	// Query the database for the lowest, "new" deploy, claim it,
	// find any subsequent deploys for the same image, pick the latest
	// and make the rest of the deploys for the same image as "skipped"
	var (
		orgID      string
		deployment Deployment
	)
	err := tx(db, func(tx *sql.Tx) error {
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
