package main

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/certifi/gocertifi"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"golang.org/x/net/context"
)

var (
	dbRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scope",
		Name:      "database_request_duration_seconds",
		Help:      "Time spent (in seconds) doing database requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})
	postRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "scope",
		Name:      "post_request_duration_seconds",
		Help:      "Time spent (in seconds) doing post requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})
)

type user struct {
	ID        *string
	Email     *string
	CreatedAt *time.Time
}

type membership struct {
	UserID     string
	InstanceID string
}

type instance struct {
	ID        *string
	Name      *string
	CreatedAt *time.Time
}

func init() {
	prometheus.MustRegister(dbRequestDuration)
	prometheus.MustRegister(postRequestDuration)
}

func main() {
	var (
		databaseURI = flag.String("database-uri", "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)")
		period      = flag.Duration("period", 10*time.Minute, "Period with which to post the DB to endpoint.")
		endpoint    = flag.String("endpoint", "https://bi.weave.works/import/service/", "Base URL to post the users to; will have table name added")
		listen      = flag.String("listen", ":80", "Port to listen on (to serve metrics)")
		logLevel    = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
	)
	flag.Parse()
	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	u, err := url.Parse(*databaseURI)
	if err != nil {
		log.Fatal(err)
		return
	}
	if u.Scheme != "postgres" {
		log.Fatal(databaseURI)
		return
	}
	db, err := sql.Open(u.Scheme, *databaseURI)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Use certifi certificates
	certPool, err := gocertifi.CACerts()
	if err != nil {
		log.Fatal(err)
		return
	}
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    certPool,
				ServerName: "bi.weave.works",
			},
		},
	}

	go func() {
		log.Infof("Listening on %s", *listen)
		mux := http.NewServeMux()
		mux.Handle("/metrics", prometheus.Handler())
		log.Fatal(http.ListenAndServe(*listen, mux))
	}()

	for ticker := time.Tick(*period); ; <-ticker {
		ctx := context.Background()

		for _, getter := range []struct {
			ty  string
			get func(*sql.DB) ([]interface{}, error)
		}{
			{"users", getUsers},
			{"memberships", getMemberships},
			{"instances", getInstances},
		} {
			var objs []interface{}
			if err := instrument.TimeRequestHistogram(ctx, getter.ty, dbRequestDuration, func(_ context.Context) error {
				var err error
				objs, err = getter.get(db)
				return err
			}); err != nil {
				log.Printf("Error getting %s: %v", getter.ty, err)
				continue
			}

			if err := instrument.TimeRequestHistogram(ctx, getter.ty, postRequestDuration, func(_ context.Context) error {
				return post(client, objs, *endpoint+getter.ty)
			}); err != nil {
				log.Printf("Error posting %s: %v", getter.ty, err)
				continue
			}

			log.Printf("Uploaded %d %s to %s", len(objs), getter.ty, *endpoint+getter.ty)
		}
	}
}

func getUsers(db *sql.DB) ([]interface{}, error) {
	rows, err := db.Query(`
SELECT
	users.id as ID,
	users.email as Email,
	users.created_at as CreatedAt
FROM users
WHERE users.deleted_at is null
ORDER BY users.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []interface{}{}
	for rows.Next() {
		user := user{}
		if err := rows.Scan(
			&user.ID, &user.Email, &user.CreatedAt,
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

func getMemberships(db *sql.DB) ([]interface{}, error) {
	rows, err := db.Query(`
SELECT
	memberships.user_id as UserID,
	memberships.organization_id as InstanceID
FROM memberships
WHERE memberships.deleted_at is null
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberships := []interface{}{}
	for rows.Next() {
		membership := membership{}
		if err := rows.Scan(
			&membership.UserID, &membership.InstanceID,
		); err != nil {
			return nil, err
		}
		memberships = append(memberships, membership)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return memberships, nil
}

func getInstances(db *sql.DB) ([]interface{}, error) {
	rows, err := db.Query(`
SELECT
	organizations.id as ID,
	organizations.name as Name,
	organizations.created_at as CreatedAt
FROM organizations
WHERE organizations.deleted_at is null
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	instances := []interface{}{}
	for rows.Next() {
		instance := instance{}
		if err := rows.Scan(
			&instance.ID, &instance.Name, &instance.CreatedAt,
		); err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return instances, nil
}

func post(client http.Client, objs []interface{}, endpoint string) error {
	buf := bytes.Buffer{}
	if err := json.NewEncoder(&buf).Encode(objs); err != nil {
		return err
	}
	resp, err := client.Post(endpoint, "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Status not okay: '%v'", resp)
	}
	return nil
}
