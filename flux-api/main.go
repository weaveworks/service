package main

import (
	"flag"
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

	billing "github.com/weaveworks/billing-client"
	"github.com/weaveworks/service/common/tracing"
	"github.com/weaveworks/service/flux-api/bus"
	"github.com/weaveworks/service/flux-api/bus/nats"
	"github.com/weaveworks/service/flux-api/db"
	"github.com/weaveworks/service/flux-api/history"
	historysql "github.com/weaveworks/service/flux-api/history/sql"
	httpserver "github.com/weaveworks/service/flux-api/http"
	"github.com/weaveworks/service/flux-api/instance"
	instancedb "github.com/weaveworks/service/flux-api/instance/sql"
	"github.com/weaveworks/service/flux-api/notifications"
	"github.com/weaveworks/service/flux-api/server"
)

const (
	shutdownTimeout = 30 * time.Second
)

var version string

type Config struct {
	listenAddr            string
	databaseSource        string
	databaseMigrationsDir string
	natsURL               string
	versionFlag           bool
	eventsURL             string
	enableBilling         bool

	billingConfig billing.Config
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.listenAddr, "listen", ":3030", "Listen address for Flux API clients")
	f.StringVar(&c.databaseSource, "database-source", "file://fluxy.db", `Database source name; includes the DB driver as the scheme. The default is a temporary, file-based DB`)
	f.StringVar(&c.databaseMigrationsDir, "database-migrations", "./flux-api/db/migrations/postgres", "Path to database migration scripts, which are in subdirectories named for each driver")
	f.StringVar(&c.natsURL, "nats-url", "", `URL on which to connect to NATS, or empty to use the standalone message bus (e.g., "nats://user:pass@nats:4222")`)
	f.BoolVar(&c.versionFlag, "version", false, "Get version number")
	f.StringVar(&c.eventsURL, "events-url", notifications.DefaultURL, "URL to which events will be sent")
	f.BoolVar(&c.enableBilling, "enable-billing", false, "Report each event to the billing system.")
}

func main() {

	traceCloser := tracing.Init("flux-api")
	defer traceCloser.Close()

	// Flag domain.
	fs := flag.NewFlagSet("default", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "DESCRIPTION\n")
		fmt.Fprintf(os.Stderr, "  fluxsvc is a deployment service.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "FLAGS\n")
		fs.PrintDefaults()
	}

	cfg := Config{}
	cfg.RegisterFlags(fs)

	// Copied from billing.Config.RegisterFlags because this uses a different flag library
	fs.IntVar(&cfg.billingConfig.MaxBufferedEvents, "billing.max-buffered-events", 1024, "Maximum number of billing events to buffer in memory")
	fs.DurationVar(&cfg.billingConfig.RetryDelay, "billing.retry-delay", 500*time.Millisecond, "How often to retry sending events to the billing ingester.")
	fs.StringVar(&cfg.billingConfig.IngesterHostPort, "billing.ingester", "localhost:24225", "points to the billing ingester sidecar (should be on localhost)")

	fs.Parse(os.Args[1:])

	if version == "" {
		version = "unversioned"
	}
	if cfg.versionFlag {
		fmt.Println(version)
		os.Exit(0)
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
		u, err := url.Parse(cfg.databaseSource)
		if err == nil {
			version, err = db.Migrate(cfg.databaseSource, cfg.databaseMigrationsDir)
		}

		if err != nil {
			logger.Log("stage", "db init", "err", err)
			os.Exit(1)
		}
		dbDriver = u.Scheme
		logger.Log("migrations", "success", "driver", dbDriver, "db-version", fmt.Sprintf("%d", version))
	}

	var messageBus bus.MessageBus
	{
		if cfg.natsURL != "" {
			bus, err := nats.NewMessageBus(cfg.natsURL)
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
		db, err := historysql.NewSQL(dbDriver, cfg.databaseSource)
		if err != nil {
			logger.Log("component", "history", "err", err)
			os.Exit(1)
		}
		historyDB = history.InstrumentedDB(db)
	}

	// Configuration, i.e., whether services are automated or not.
	var instanceDB instance.ConnectionDB
	{
		db, err := instancedb.New(dbDriver, cfg.databaseSource)
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

	var billingClient server.BillingClient
	if cfg.enableBilling {
		var err error
		billingClient, err = billing.NewClient(cfg.billingConfig)
		if err != nil {
			logger.Log("component", "billing", "err", err)
			os.Exit(1)
		}
	} else {
		billingClient = server.NoopBillingClient{}
	}

	// The server.
	server := server.New(version, instancer, instanceDB, messageBus, logger, cfg.eventsURL, billingClient)

	// Mechanical components.
	errc := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	// HTTP transport component.
	go func() {
		logger.Log("addr", cfg.listenAddr)
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		httpServer := httpserver.NewServer(server, server, server, logger)
		handler := httpServer.MakeHandler(httpserver.NewServiceRouter())
		mux.Handle("/", handler)
		mux.Handle("/api/flux/", http.StripPrefix("/api/flux", handler))
		operationNameFunc := nethttp.OperationNameFunc(func(r *http.Request) string {
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		})
		errc <- http.ListenAndServe(cfg.listenAddr, nethttp.Middleware(opentracing.GlobalTracer(), mux, operationNameFunc))
	}()

	logger.Log("exiting", <-errc)
}
