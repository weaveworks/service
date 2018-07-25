package main

import (
	"flag"
	"net/http"

	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/billing-enforcer/job"
	"github.com/weaveworks/service/common/users"
)

var jobCollector = instrument.NewJobCollector("billing")

func init() {
	jobCollector.Register()
}

const index = `
<html>
	<head><title>Billing Enforcer</title></head>
	<body>
		<h1>Billing Enforcer</h1>
		<ul>
			<li>
				<form action="enforcer/enforce" method="post">
				<input type="hidden" name="csrf_token" value="$__CSRF_TOKEN_PLACEHOLDER__">
				<button type="submit">Trigger Enforce</button>
				</form>
			</li>
		</ul>
	</body>
</html>
`

func main() {
	var (
		cronSpec = flag.String(
			"cron-spec",
			// Hourly at xx:40 - Seconds, Minutes, Hours, Day of month, Month, Day of week
			"0 40  * * *",
			"Cron spec for periodic enforcement tasks.")

		serverConfig server.Config
		usersConfig  users.Config
		cfg          job.Config
	)
	cfg.RegisterFlags(flag.CommandLine)
	serverConfig.RegisterFlags(flag.CommandLine)
	usersConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()

	// Set up server first as it sets up logging as a side-effect
	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initialising server: %v", err)
	}
	defer server.Shutdown()

	users, err := users.NewClient(usersConfig)
	if err != nil {
		log.Fatalf("error initialising users client: %v", err)
	}

	c := cron.New()
	enforceJob := job.NewEnforce(users, cfg, jobCollector)
	c.AddJob(*cronSpec, enforceJob)
	c.Start()
	defer c.Stop()

	// Run job at startup
	go enforceJob.Run()

	server.HTTP.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(index))
	})
	server.HTTP.Path("/enforce").Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := enforceJob.Do(); err != nil {
			w.Write([]byte(err.Error()))
		} else {
			w.Write([]byte("Success"))
		}
	})
	server.Run()
}
