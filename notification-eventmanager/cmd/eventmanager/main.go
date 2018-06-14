package main

import (
	"flag"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/notification-eventmanager/db"
	"github.com/weaveworks/service/notification-eventmanager/eventmanager"
	"github.com/weaveworks/service/notification-eventmanager/sqsconnect"
	"github.com/weaveworks/service/notification-eventmanager/types"
	usersClient "github.com/weaveworks/service/users/client"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace:              "notification",
			ServerGracefulShutdownTimeout: 16 * time.Second,
		}
		dbConfig dbconfig.Config
		logLevel string
		sqsURL   string
		// Connect to users service to get information about an event's instance
		usersServiceURL string
		eventTypesPath  string
		wcURL           string
	)

	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine, "", "URI where the database can be found", "", "Path where the database migration files can be found")

	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&sqsURL, "sqsURL", "sqs://123user:123password@localhost:9324/events", "URL to connect to SQS")
	flag.StringVar(&usersServiceURL, "usersServiceURL", "users.default:4772", "URL to connect to users service")
	flag.StringVar(&eventTypesPath, "eventtypes", "", "Path to a JSON file defining available event types")
	flag.StringVar(&wcURL, "wc.url", "https://cloud.weave.works/", "Weave Cloud URL")

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

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	em := eventmanager.New(uclient, db, sqsCli, sqsQueue, wcURL)

	if eventTypesPath != "" {
		eventTypes, err := types.EventTypesFromFile(eventTypesPath)
		if err != nil {
			log.Fatalf("Cannot get event types from file %s: %s", eventTypesPath, err)
		}
		log.Infof("Synchronizing %d event types with DB", len(eventTypes))
		err = em.DB.SyncEventTypes(eventTypes)
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
