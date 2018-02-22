package main

import (
	"database/sql"
	"flag"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common/dbwait"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/notification-eventmanager/db/postgres"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager"
	"github.com/weaveworks/service/notification-eventmanager/sqsconnect"
	"github.com/weaveworks/service/notification-eventmanager/types"
	usersClient "github.com/weaveworks/service/users/client"
	"gopkg.in/mattes/migrate.v1/migrate"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace:              "notification",
			ServerGracefulShutdownTimeout: 16 * time.Second,
		}
		logLevel string
		sqsURL   string
		// Connect to users service to get information about an event's instance
		usersServiceURL string
		databaseURI     string
		migrationsDir   string
		eventTypesPath  string
	)

	serverConfig.RegisterFlags(flag.CommandLine)

	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&sqsURL, "sqsURL", "sqs://123user:123password@localhost:9324/events", "URL to connect to SQS")
	flag.StringVar(&usersServiceURL, "usersServiceURL", "users.default:4772", "URL to connect to users service")
	flag.StringVar(&databaseURI, "database.uri", "", "URI where the database can be found")
	flag.StringVar(&migrationsDir, "database.migrations", "", "Path where the database migration files can be found")
	flag.StringVar(&eventTypesPath, "eventtypes", "", "Path to a JSON file defining available event types")

	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
	}

	sqsCli, sqsQueue, err := sqsconnect.NewSQS(sqsURL)
	if err != nil {
		log.Fatalf("cannot connect to SQS %q, error: %s", sqsURL, err)
	}

	var uclient *users.Client
	if usersServiceURL == "mock" {
		uclient = &users.Client{UsersClient: usersClient.MockClient{}}
	} else {
		uclient, err = users.NewClient(users.Config{HostPort: usersServiceURL})
		if err != nil {
			log.Fatalf("cannot create users client: %v, error: %s", usersServiceURL, err)
		}
	}

	if databaseURI == "" {
		log.Fatal("Database URI is required")
	}

	db, err := sql.Open("postgres", databaseURI)
	if err != nil {
		log.Fatalf("cannot open postgres URI %s, error: %s", databaseURI, err)
	}

	if err := dbwait.Wait(db); err != nil {
		log.Fatalf("cannot establish db connection, error: %s", err)
	}

	if migrationsDir != "" {
		log.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(databaseURI, migrationsDir); !ok {
			for _, err := range errs {
				log.Error(err)
			}
			log.Fatalf("database migrations failed: %s", err)
		}
	}
	psql, err := postgres.New(databaseURI, migrationsDir)

	if err != nil {
		log.Fatalf("Its dead fam %s", err)
	}
	em := eventmanager.New(uclient, db, sqsCli, sqsQueue, psql)

	if eventTypesPath != "" {
		eventTypes, err := types.EventTypesFromFile(eventTypesPath)
		if err != nil {
			log.Fatalf("Cannot get event types from file %s: %s", eventTypesPath, err)
		}
		log.Infof("Synchronizing %d event types with DB", len(eventTypes))
		err = em.SyncEventTypes(eventTypes)
		if err != nil {
			log.Fatalf("Cannot synchronize event types: %s", err)
		}
		log.Infof("Synchronized event types")
	}

	log.Info("listening for requests")
	s, err := server.New(serverConfig)
	if err != nil {
		log.Fatal(err)
	}

	em.Register(s.HTTP)

	defer func() {
		s.Shutdown()
		em.Wait()
	}()
	s.Run()
}
