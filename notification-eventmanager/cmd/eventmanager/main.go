package main

import (
	"flag"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/notification-eventmanager"
	"github.com/weaveworks/service/notification-eventmanager/sqsconnect"
	usersClient "github.com/weaveworks/service/users/client"
)

// NewConfig returns new config for event manager with connection to SQS and a connection to users service
func newConfig(queueURL, configManagerURL, usersServiceURL string) (eventmanager.Config, error) {
	sqsCli, sqsQueue, err := sqsconnect.NewSQS(queueURL)
	if err != nil {
		return eventmanager.Config{}, errors.Wrapf(err, "cannot connect to SQS %q", queueURL)
	}

	var uclient *users.Client
	if usersServiceURL == "mock" {
		uclient = &users.Client{UsersClient: usersClient.MockClient{}}
	} else {
		uclient, err = users.NewClient(users.Config{HostPort: usersServiceURL})
		if err != nil {
			return eventmanager.Config{}, errors.Wrapf(err, "cannot create users client: %v", usersServiceURL)
		}
	}

	return eventmanager.Config{
		ConfigManager: &eventmanager.ConfigClient{URL: configManagerURL},
		SQSClient:     sqsCli,
		SQSQueue:      sqsQueue,
		UsersClient:   uclient,
	}, nil
}

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace:              "notification",
			ServerGracefulShutdownTimeout: 16 * time.Second,
		}
		logLevel         string
		sqsURL           string
		configManagerURL string
		// Connect to users service to get information about an event's instance
		usersServiceURL string
	)

	serverConfig.RegisterFlags(flag.CommandLine)

	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&sqsURL, "sqsURL", "sqs://123user:123password@localhost:9324/events", "URL to connect to SQS")
	flag.StringVar(&configManagerURL, "configManagerURL", "", "URL to connect to config managerr")
	flag.StringVar(&usersServiceURL, "usersServiceURL", "users.default:4772", "URL to connect to users service")
	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	var cfg eventmanager.Config
	cfg, err := newConfig(sqsURL, configManagerURL, usersServiceURL)
	if err != nil {
		log.Fatalf("cannot create config for event manager, error: %s", err)
	}
	em := eventmanager.New(cfg)

	log.Info("listening for requests")
	s, err := server.New(serverConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Internal API
	s.HTTP.HandleFunc("/api/notification/testevent", em.RateLimited(em.TestEventHandler)).Methods("POST")
	s.HTTP.HandleFunc("/api/notification/events", em.RateLimited(em.EventHandler)).Methods("POST")
	s.HTTP.HandleFunc("/api/notification/slack/{instanceID}/{eventType}", em.RateLimited(em.SlackHandler)).Methods("POST")
	s.HTTP.HandleFunc("/api/notification/events/healthcheck", em.HandleHealthCheck).Methods("GET")

	// External API - reachable from outside Weave Cloud cluster
	s.HTTP.HandleFunc("/api/notification/external/events", em.RateLimited(em.EventHandler)).Methods("POST")

	defer func() {
		s.Shutdown()
		em.Wait()
	}()
	s.Run()
}
