package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/billing-aggregator/job"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/common/bigquery"
	"github.com/weaveworks/service/common/dbconfig"
)

var jobCollector = instrument.NewJobCollector("billing")

func init() {
	jobCollector.Register()
}

const index = `
<html>
	<head><title>Billing Aggregator</title></head>
	<body>
		<h1>Billing Aggregator</h1>
			<form action="aggregator/aggregate" method="post">
				<input type="hidden" name="csrf_token" value="$__CSRF_TOKEN_PLACEHOLDER__">
				<p>
					<label>Manually re-aggregate and import usage since: <br />
					<input type="text" name="since" /> (Format <code>2018-01-01T15:00:00+00:00</code>)
					</label>
				</p>
				<button type="submit">Trigger Aggregation</button>
			</form>
	</body>
</html>
`

func main() {
	var (
		cronSpec = flag.String(
			"cron-spec",
			"0 10 * * * *", // Hourly at 10 minutes past - Seconds, Minutes, Hours, Day of month, Month, Day of week
			"Cron spec for periodic query execution.")
		serverConfig   server.Config
		bigQueryConfig bigquery.Config
		dbConfig       dbconfig.Config
	)
	serverConfig.RegisterFlags(flag.CommandLine)
	bigQueryConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine, "postgres://postgres@billing-db/billing?sslmode=disable", "Database to use.", "/migrations", "Migrations directory.")
	flag.Parse()

	if err := logging.Setup(serverConfig.LogLevel.String()); err != nil {
		log.Fatalf("Error initialising logging: %v", err)
	}
	serverConfig.Log = logging.Logrus(log.StandardLogger())

	bigqueryClient, err := bigquery.New(context.Background(), bigQueryConfig)
	if err != nil {
		log.Fatalf("Error initialising BigQuery client: %v", err)
	}

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initialising database client: %v", err)
	}
	defer db.Close(context.Background())

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initialising server: %v", err)
	}
	defer server.Shutdown()

	c := cron.New()
	job := job.NewAggregate(bigqueryClient, db, jobCollector)
	c.AddJob(*cronSpec, job)
	c.Start()
	defer c.Stop()

	// Do a aggregation on startup
	go job.Run()

	server.HTTP.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index))
	})
	server.HTTP.Path("/aggregate").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var since *time.Time
		if s := r.FormValue("since"); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			since = &t
		}

		if err := job.Do(since); err != nil {
			w.Write([]byte(err.Error()))
		} else {
			w.Write([]byte("Success"))
		}
	})
	// healthCheck handles a very simple health check
	server.HTTP.Path("/healthcheck").Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	server.Run()
}
