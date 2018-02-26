package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"

	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/common/tracing"
	"github.com/weaveworks/service/flux-api/bus"
	"github.com/weaveworks/service/flux-api/bus/nats"
	"github.com/weaveworks/service/flux-api/config"
	"github.com/weaveworks/service/flux-api/db"
	"github.com/weaveworks/service/flux-api/history"
	historysql "github.com/weaveworks/service/flux-api/history/sql"
	httpserver "github.com/weaveworks/service/flux-api/http"
	"github.com/weaveworks/service/flux-api/instance"
	instancedb "github.com/weaveworks/service/flux-api/instance/sql"
	"github.com/weaveworks/service/flux-api/server"
)

const shutdownTimeout = 30 * time.Second

var version string

func main() {

	traceCloser := tracing.Init("flux-api")
	defer traceCloser.Close()

	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxsvc is a deployment service.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	var (
		listenAddr            = fs.StringP("listen", "l", ":3030", "Listen address for Flux API clients")
		databaseSource        = fs.String("database-source", "file://fluxy.db", `Database source name; includes the DB driver as the scheme. The default is a temporary, file-based DB`)
		databaseMigrationsDir = fs.String("database-migrations", "./flux-api/db/migrations", "Path to database migration scripts, which are in subdirectories named for each driver")
		natsURL               = fs.String("nats-url", "", `URL on which to connect to NATS, or empty to use the standalone message bus (e.g., "nats://user:pass@nats:4222")`)
		versionFlag           = fs.Bool("version", false, "Get version number")
		eventsURL             = fs.String("events-url", "", "URL to which events will be sent, or empty to use instance-specific Slack settings")
	)
	fs.Parse(os.Args)

	if version == "" {
		version = "unversioned"
	}
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// If the events-url flag is present, ignore instance specific Slack settings.
	var defaultEventsConfig *instance.Config
	if *eventsURL != "" {
		defaultEventsConfig = &instance.Config{
			Settings: config.Instance{
				Slack: config.Notifier{
					HookURL: *eventsURL,
					NotifyEvents: []string{
						event.EventRelease,
						event.EventAutoRelease,
						event.EventSync,
					},
				},
			},
		}
	}

	// Logger component.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	// Initialise database; we must fail if we can't do this, because
	// most things depend on it.
	var dbDriver string
	{
		var version uint64
		u, err := url.Parse(*databaseSource)
		if err == nil {
			version, err = db.Migrate(*databaseSource, *databaseMigrationsDir)
		}

		if err != nil {
			logger.Log("stage", "db init", "err", err)
			os.Exit(1)
		}
		dbDriver = db.DriverForScheme(u.Scheme)
		logger.Log("migrations", "success", "driver", dbDriver, "db-version", fmt.Sprintf("%d", version))
	}

	var messageBus bus.MessageBus
	{
		if *natsURL != "" {
			bus, err := nats.NewMessageBus(*natsURL)
			if err != nil {
				logger.Log("component", "message bus", "err", err)
				os.Exit(1)
			}
			logger.Log("component", "message bus", "type", "NATS")
			messageBus = bus
		} else {
			logger.Log("component", "message bus", "err", "not configured")
			os.Exit(1)
		}
	}

	var historyDB history.DB
	{
		db, err := historysql.NewSQL(dbDriver, *databaseSource)
		if err != nil {
			logger.Log("component", "history", "err", err)
			os.Exit(1)
		}
		historyDB = history.InstrumentedDB(db)
	}

	// Configuration, i.e., whether services are automated or not.
	var instanceDB instance.DB
	{
		db, err := instancedb.New(dbDriver, *databaseSource)
		if err != nil {
			logger.Log("component", "config", "err", err)
			os.Exit(1)
		}
		instanceDB = instance.InstrumentedDB(db)
	}

	var instancer instance.Instancer
	{
		// Instancer, for the instancing of operations
		instancer = &instance.MultitenantInstancer{
			DB:        instanceDB,
			Connecter: messageBus,
			Logger:    logger,
			History:   historyDB,
		}
	}

	// The server.
	server := server.New(version, instancer, instanceDB, messageBus, logger, defaultEventsConfig)

	// Mechanical components.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// HTTP transport component.
	go func() {
		logger.Log("addr", *listenAddr)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		httpServer := httpserver.NewServer(server, server, server)
		handler := httpServer.MakeHandler(httpserver.NewServiceRouter(), logger)
		mux.Handle("/", handler)
		mux.Handle("/api/flux/", http.StripPrefix("/api/flux", handler))
		operationNameFunc := nethttp.OperationNameFunc(func(r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		})
		errc <- http.ListenAndServe(*listenAddr, nethttp.Middleware(opentracing.GlobalTracer(), mux, operationNameFunc))
	}()

	logger.Log("exiting", <-errc)
}
