package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-uploader/job"
	"github.com/weaveworks/service/billing-uploader/job/usage"
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
				<form action="uploader/upload" method="post">
				<input type="hidden" name="csrf_token" value="$__CSRF_TOKEN_PLACEHOLDER__">
				<button type="submit">Trigger Upload</button>
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
			"0 15 2 * * *", // Daily at 02:15:00 - Seconds, Minutes, Hours, Day of month, Month, Day of week
			"Cron spec for periodic execution of the Zuora uploader job. Should be scheduled to run once per day")
		uploadGCPCronSpec = flag.String(
			"upload-gcp-cron-spec",
			"0 0 * * * *", // Hourly at :00 - Seconds, Minutes, Hours, Day of month, Month, Day of week
			"Cron spec for periodic execution of the GCP uploader job. Should be scheduled to run once an hour")
		invoiceCronSpec = flag.String(
			"invoice-cron-spec",
			"0 * * * * *", // Every minute
			"Cron spec for periodic execution of the invoice job")
		logLevel     = flag.String("log.level", "info", "The log level")
		serverConfig server.Config
		dbConfig     db.Config
		usersConfig  users.Config
		zuoraConfig  zuora.Config
		gcpConfig    control.Config
	)
	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine)
	usersConfig.RegisterFlags(flag.CommandLine)
	zuoraConfig.RegisterFlags(flag.CommandLine)
	gcpConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error initialising logging: %v", err)
	}

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initialising database client: %v", err)
	}
	defer db.Close(context.Background())

	users, err := users.NewClient(usersConfig)
	if err != nil {
		log.Fatalf("Error initialising users client: %v", err)
	}

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initialising server: %v", err)
	}
	defer server.Shutdown()

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
	invoice := job.NewInvoiceUpload(db, zuora, jobCollector)
	invoiceCron.AddJob(*invoiceCronSpec, invoice)
	invoiceCron.Start()
	defer invoiceCron.Stop()

	server.HTTP.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index))
	})
	server.HTTP.Path("/upload").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "usage /upload/gcp or /upload/zuora", http.StatusSeeOther)
	})
	server.HTTP.Path("/upload/gcp").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := gcpUpload.Do(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write([]byte("Success"))
		}
	})
	server.HTTP.Path("/upload/zuora").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := zuoraUpload.Do(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write([]byte("Success"))
		}
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
