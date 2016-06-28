package main

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/certifi/gocertifi"
	_ "github.com/lib/pq"
)

type user struct {
	ID                    *string
	Email                 *string
	TokenCreatedAt        *time.Time
	ApprovedAt            *time.Time
	FirstLoginAt          *time.Time
	CreatedAt             *time.Time
	OrgID                 *string
	OrgName               *string
	OrgFirstProbeUpdateAt *time.Time
	OrgCreatedAt          *time.Time
}

func main() {
	var (
		databaseURI = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		period      = flag.Duration("period", 10*time.Minute, "Period with which to post the DB to endpoint.")
		endpoint    = flag.String("endpoint", "https://bi.weave.works/import/service/users", "Endpoint to post the users to.")
	)
	flag.Parse()

	u, err := url.Parse(*databaseURI)
	if err != nil {
		log.Fatal(err)
	}
	if u.Scheme != "postgres" {
		log.Fatal(databaseURI)
	}
	db, err := sql.Open(u.Scheme, *databaseURI)
	if err != nil {
		log.Fatal(err)
	}

	// Use certifi certificates
	certPool, err := gocertifi.CACerts()
	if err != nil {
		panic(err)
	}
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    certPool,
				ServerName: "bi.weave.works",
			},
		},
	}

	for ticker := time.Tick(*period); ; <-ticker {
		users, err := getUsers(db)
		if err != nil {
			log.Printf("Error getting users: %v", err)
			return
		}
		if err := postUsers(client, users, *endpoint); err != nil {
			log.Printf("Error posting users: %v", err)
		}
		log.Printf("Uploaded %d records to %s", len(users), *endpoint)
	}
}

func getUsers(db *sql.DB) ([]user, error) {
	rows, err := db.Query(`
SELECT
	users.id as ID,
	users.email as Email,
	users.token_created_at as TokenCreatedAt,
	users.approved_at as ApprovedAt,
	users.first_login_at as FirstLoginAt,
	users.created_at as CreatedAt,
	users.organization_id as OrgID,
	organizations.name as OrgName,
	organizations.first_probe_update_at as OrgFirstProbeUpdateAt,
	organizations.created_at as OrgCreatedAt
FROM users
LEFT JOIN memberships on (memberships.user_id = users.id)
LEFT JOIN organizations on (memberships.organization_id = organizations.id)
WHERE users.deleted_at is null
ORDER BY users.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []user{}
	for rows.Next() {
		user := user{}
		if err := rows.Scan(
			&user.ID, &user.Email, &user.TokenCreatedAt, &user.ApprovedAt, &user.FirstLoginAt,
			&user.CreatedAt, &user.OrgID, &user.OrgName, &user.OrgFirstProbeUpdateAt,
			&user.OrgCreatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return users, nil
}

func postUsers(client http.Client, users []user, endpoint string) error {
	buf := bytes.Buffer{}
	if err := json.NewEncoder(&buf).Encode(users); err != nil {
		return err
	}
	resp, err := client.Post(endpoint, "application/json", &buf)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Status not okay: '%v'", resp)
	}
	return nil
}
