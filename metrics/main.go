package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/certifi/gocertifi"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/dbconfig"

	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/filter"
)

var (
	postRequestCollector = instrument.NewHistogramCollector(prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: common.PrometheusNamespace,
		Name:      "post_request_duration_seconds",
		Help:      "Time spent (in seconds) doing post requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"}))
)

func init() {
	postRequestCollector.Register()
}

type bqUser struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

type bqInstance struct {
	ID                   string
	ExternalID           string
	Name                 string
	CreatedAt            time.Time
	FirstSeenConnectedAt *time.Time
	Platform             string
	Environment          string
	BillingProvider      string
	RefuseDataAccess     string
	RefuseDataUpload     string
}

type bqMembership struct {
	UserID     string
	InstanceID string
}

func main() {
	var (
		dbCfg    dbconfig.Config
		period   = flag.Duration("period", 10*time.Minute, "Period with which to post the DB to endpoint.")
		endpoint = flag.String("endpoint", "https://bi.weave.works/import/service/", "Base URL to post the users to; will have table name added")
		listen   = flag.String("listen", ":80", "Port to listen on (to serve metrics)")
		logLevel = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
	)
	dbCfg.RegisterFlags(flag.CommandLine, "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)", "", "Migrations directory.")
	flag.Parse()
	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	d := db.MustNew(dbCfg)

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
		instrument.CollectedRequest(context.Background(), "Metrics upload", postRequestCollector, nil, func(ctx context.Context) error {
			for _, getter := range []struct {
				ty  string
				get func(context.Context, db.DB) ([]interface{}, error)
			}{
				{"users", getUsers},
				{"memberships", getMemberships},
				{"instances", getInstances},
			} {
				objs, err := getter.get(ctx, d)
				if err != nil {
					log.Printf("Error getting %s: %v", getter.ty, err)
					continue
				}

				if err := instrument.CollectedRequest(ctx, getter.ty, postRequestCollector, nil, func(_ context.Context) error {
					return post(client, objs, *endpoint+getter.ty)
				}); err != nil {
					log.Printf("Error posting %s: %v", getter.ty, err)
					continue
				}

				log.Printf("Uploaded %d %s to %s", len(objs), getter.ty, *endpoint+getter.ty)
			}
			return nil
		})
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func getUsers(ctx context.Context, d db.DB) ([]interface{}, error) {
	users, err := d.ListUsers(ctx, filter.All, 0)
	if err != nil {
		return nil, err
	}
	results := []interface{}{}
	for _, user := range users {
		result := bqUser{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		}
		results = append(results, result)
	}
	return results, nil
}

func getMemberships(ctx context.Context, d db.DB) ([]interface{}, error) {
	memberships, err := d.ListMemberships(ctx)
	if err != nil {
		return nil, err
	}
	results := []interface{}{}
	for _, membership := range memberships {
		result := bqMembership{
			UserID:     membership.UserID,
			InstanceID: membership.OrganizationID,
		}
		results = append(results, result)
	}
	return results, nil
}

func getInstances(ctx context.Context, d db.DB) ([]interface{}, error) {
	instances, err := d.ListOrganizations(ctx, filter.All, 0)
	if err != nil {
		return nil, err
	}
	results := []interface{}{}
	for _, instance := range instances {
		result := bqInstance{
			ID:                   instance.ID,
			ExternalID:           instance.ExternalID,
			Name:                 instance.Name,
			CreatedAt:            instance.CreatedAt,
			FirstSeenConnectedAt: instance.FirstSeenConnectedAt,
			Platform:             instance.Platform,
			Environment:          instance.Environment,
			BillingProvider:      instance.BillingProvider(),
			RefuseDataAccess:     boolString(instance.RefuseDataAccess),
			RefuseDataUpload:     boolString(instance.RefuseDataUpload),
		}
		results = append(results, result)
	}
	return results, nil
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
