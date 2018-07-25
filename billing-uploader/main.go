package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-uploader/job"
	"github.com/weaveworks/service/billing-uploader/job/usage"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/common/gcp/control"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/common/zuora"
)

var jobCollector = instrument.NewJobCollector("billing")

const index = `
<html>
	<head><title>Billing Uploader</title></head>
	<body>
		<h1>Billing Uploader</h1>
		<ul>
			<li>
				<form action="uploader/upload/gcp" method="post">
				<input type="hidden" name="csrf_token" value="$__CSRF_TOKEN_PLACEHOLDER__">
				<button type="submit">Google Cloud Platform</button>
				</form>
			</li>
			<li>
				<form action="uploader/upload/zuora" method="post">
				<input type="hidden" name="csrf_token" value="$__CSRF_TOKEN_PLACEHOLDER__">
				<button type="submit">Zuora</button>
				</form>
			</li>
		</ul>
	</body>
</html>
`

func init() {
	jobCollector.Register()
}

func main() {
	var (
		uploadZuoraCronSpec = flag.String(
			"upload-zuora-cron-spec",
			"0 30 2 * * *", // Daily at 02:30:00 - Seconds, Minutes, Hours, Day of month, Month, Day of week
			"Cron spec for periodic execution of the Zuora uploader job. Should be scheduled to run once per day")
		uploadGCPCronSpec = flag.String(
			"upload-gcp-cron-spec",
			// It is scheduled to go hourly at :15 because the aggregation of usage is scheduled at :10
			"0 15 * * * *", // Seconds, Minutes, Hours, Day of month, Month, Day of week
			"Cron spec for periodic execution of the GCP uploader job. Should be scheduled to run once an hour")
		invoiceCronSpec = flag.String(
			"invoice-cron-spec",
			"0 * * * * *", // Every minute
			"Cron spec for periodic execution of the invoice job")
		serverConfig server.Config
		dbConfig     dbconfig.Config
		usersConfig  users.Config
		zuoraConfig  zuora.Config
		gcpConfig    control.Config
	)
	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine, "postgres://postgres@billing-db/billing?sslmode=disable", "Database to use.", "/migrations", "Migrations directory.")
	usersConfig.RegisterFlags(flag.CommandLine)
	zuoraConfig.RegisterFlags(flag.CommandLine)
	gcpConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()

	// Set up server first as it sets up logging as a side-effect
	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initialising server: %v", err)
	}
	defer server.Shutdown()

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initialising database client: %v", err)
	}
	defer db.Close(context.Background())

	users, err := users.NewClient(usersConfig)
	if err != nil {
		log.Fatalf("Error initialising users client: %v", err)
	}

	// Zuora upload cron
	zuora := zuora.New(zuoraConfig, nil)
	zuoraCron, zuoraUpload := startCron(*uploadZuoraCronSpec, db, users, usage.NewZuora(zuora), jobCollector)
	defer zuoraCron.Stop()

	// GCP upload cron
	var gcpCron *cron.Cron
	var gcpUpload *job.UsageUpload
	if gcpConfig.ServiceAccountKeyFile != "" {
		gcp, err := control.NewClient(gcpConfig)
		if err != nil {
			log.Fatalf("Error initialising GCP Control API client: %v", err)
		}
		gcpCron, gcpUpload = startCron(*uploadGCPCronSpec, db, users, usage.NewGCP(gcp), jobCollector)
		defer gcpCron.Stop()
	} else {
		log.Infof("GCP usage uploader is disabled")
	}

	invoiceCron := cron.New()
	invoice := job.NewInvoiceUpload(db, users, zuora, jobCollector)
	invoiceCron.AddJob(*invoiceCronSpec, invoice)
	invoiceCron.Start()
	defer invoiceCron.Stop()

	server.HTTP.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index))
	})
	server.HTTP.Path("/upload").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "use /upload/gcp or /upload/zuora", http.StatusSeeOther)
	})
	server.HTTP.Path("/upload/gcp").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := gcpUpload.Do(time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write([]byte("Success"))
		}
	})
	server.HTTP.Path("/upload/zuora").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := zuoraUpload.Do(time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

func startCron(cronspec string, db db.DB, u *users.Client, uploader usage.Uploader, collector *instrument.JobCollector) (*cron.Cron, *job.UsageUpload) {
	c := cron.New()
	upload := job.NewUsageUpload(db, u, uploader, jobCollector)
	c.AddJob(cronspec, upload)
	c.Start()
	return c, upload
}
