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
	"github.com/weaveworks/service/billing-uploader/job"
	"github.com/weaveworks/service/billing/db"
	billingUsers "github.com/weaveworks/service/billing/users"
	"github.com/weaveworks/service/billing/zuora"
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
		uploadCronSpec = flag.String(
			"upload-cron-spec",
			"0 15 2 * * *", // Daily at 02:15:00 - Seconds, Minutes, Hours, Day of month, Month, Day of week
			"Cron spec for periodic execution of the uploader job. Should be scheduled to run once per day")
		invoiceCronSpec = flag.String(
			"invoice-cron-spec",
			"0 * * * * *", // Every minute
			"Cron spec for periodic execution of the invoice job")
		logLevel     = flag.String("log.level", "info", "The log level")
		serverConfig server.Config
		dbConfig     db.Config
		usersConfig  billingUsers.Config
		zuoraConfig  zuora.Config
	)
	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine)
	usersConfig.RegisterFlags(flag.CommandLine)
	zuoraConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error initialising logging: %v", err)
	}

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initialising database client: %v", err)
	}
	defer db.Close(context.Background())

	users, err := billingUsers.New(usersConfig)
	if err != nil {
		log.Fatalf("error initialising users client: %v", err)
	}

	zuora := zuora.New(zuoraConfig, nil)

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initialising server: %v", err)
	}
	defer server.Shutdown()

	uploadCron := cron.New()
	upload := job.NewUsageUpload(db, users, zuora, jobCollector)
	uploadCron.AddJob(*uploadCronSpec, upload)
	uploadCron.Start()
	defer uploadCron.Stop()

	invoiceCron := cron.New()
	invoice := job.NewInvoiceUpload(db, zuora, jobCollector)
	invoiceCron.AddJob(*invoiceCronSpec, invoice)
	invoiceCron.Start()
	defer uploadCron.Stop()

	server.HTTP.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index))
	})
	server.HTTP.Path("/upload").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := upload.Do(); err != nil {
			w.Write([]byte(err.Error()))
		} else {
			w.Write([]byte("Success"))
		}
	})
	server.Run()
}
