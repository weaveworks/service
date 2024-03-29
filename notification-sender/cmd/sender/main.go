package main

import (
	"context"
	"flag"
	"net/http"

	googleLogging "cloud.google.com/go/logging"
	"github.com/gorilla/mux"
	"github.com/nats-io/go-nats"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/signals"
	"github.com/weaveworks/service/notification-eventmanager/sqsconnect"
	"github.com/weaveworks/service/notification-eventmanager/types"
	"github.com/weaveworks/service/notification-sender"
)

type stopCancel struct {
	cancel            context.CancelFunc
	stackDriverSender *sender.StackdriverSender
}

func (sc stopCancel) Stop() error {
	sc.stackDriverSender.Stop()
	sc.cancel()
	return nil
}

func main() {
	var (
		logLevel         string
		port             string
		natsURL          string
		sqsURL           string
		slackFrom        string
		stackdriverLogID string
		emailURI         string
		emailFrom        string
		emailReplyTo     string
		opsgenieURL      string
		opsgenieEUURL    string
	)

	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.StringVar(&port, "port", "80", "port for prometheus mertics")
	flag.StringVar(&natsURL, "nats", "nats://localhost:4222", "URL for NATS service")
	flag.StringVar(&sqsURL, "sqsURL", "sqs://123user:123password@localhost:9324/events", "URL to connect to SQS")
	flag.StringVar(&slackFrom, "slackFrom", "Weave Cloud", "Username for slack notifications")
	// stackdriverLogID is logID in stackdriver; it looks like "projects/{projectID}/logs/{logID}"
	// it must be less than 512 characters long and can only include the following characters:
	// upper and lower case alphanumeric characters, forward-slash, underscore, hyphen, and period.
	flag.StringVar(&stackdriverLogID, "stackdriverLogID", "WeaveCloud", "LogID for stackdriver notifications")
	flag.StringVar(&emailURI, "emailURI", "", "uri of smtp server to send email through, of the format: smtp://username:password@hostname:port. Email-uri must be provided. For local development, you can set this to: log://, which will log all emails.")
	flag.StringVar(&emailFrom, "emailFrom", "Weave Cloud <notifications@weave.works>", "From address for emails.")
	flag.StringVar(&emailReplyTo, "emailReplyTo", "Weave Cloud <support@weave.works>", "Reply-To for emails.")
	flag.StringVar(&opsgenieURL, "opsgenieURL", "https://api.opsgenie.com", "Opsgenie API URL")
	flag.StringVar(&opsgenieEUURL, "opsgenieEUURL", "https://api.eu.opsgenie.com", "Opsgenie EU API URL")

	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	if err := sender.ValidateEmailSender(emailURI, emailFrom, emailReplyTo); err != nil {
		log.Fatalf("cannot validate email sender (URI: %s, From: %s), error: %s", emailURI, emailFrom, err)
	}

	es := &sender.EmailSender{
		URI:     emailURI,
		From:    emailFrom,
		ReplyTo: emailReplyTo,
	}

	ss := &sender.SlackSender{
		Username: slackFrom,
	}

	sds := &sender.StackdriverSender{
		LogID:   stackdriverLogID,
		Clients: make(map[string]*googleLogging.Client),
	}

	ogs, err := sender.NewOpsGenie(opsgenieURL)
	if err != nil {
		log.Fatalf("error creating OpsGenieSender: %v", err)
	}

	ogsEU, err := sender.NewOpsGenie(opsgenieEUURL)
	if err != nil {
		log.Fatalf("error creating OpsGenieSender for EU region: %v", err)
	}

	pds := sender.NewPagerDutySender()

	sqsCli, sqsQueue, err := sqsconnect.NewSQS(sqsURL)
	if err != nil {
		log.Fatalf("cannot connect to SQS %q, error: %s", sqsURL, err)
	}

	natsConn, err := nats.Connect(natsURL, nats.MaxReconnects(-1))
	if err != nil {
		log.Fatalf("cannot connect to NATS %q, error: %s", natsURL, err)
	}

	bs := &sender.BrowserSender{NATS: natsConn}

	config := sender.Config{
		SQSCli:   sqsCli,
		SQSQueue: sqsQueue,
		NATS:     natsConn,
	}

	s := sender.New(config)

	s.RegisterNotifier(types.EmailReceiver, es.Send)
	s.RegisterNotifier(types.SlackReceiver, ss.Send)
	s.RegisterNotifier(types.BrowserReceiver, bs.Send)
	s.RegisterNotifier(types.StackdriverReceiver, sds.Send)
	s.RegisterNotifier(types.OpsGenieReceiver, ogs.Send)
	s.RegisterNotifier(types.OpsGenieEUReceiver, ogsEU.Send)
	s.RegisterNotifier(types.PagerDutyReceiver, pds.Send)

	ctx, cancel := context.WithCancel(context.Background())

	r := mux.NewRouter()
	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/api/notification/sender", s.HandleBrowserNotifications).Methods("GET")
	r.HandleFunc("/api/notification/sender/healthcheck", s.HandleHealthCheck).Methods("GET")

	go func() {
		log.Fatalln(http.ListenAndServe(":"+port, r))
	}()

	log.Info("Running notifications sender")

	go signals.SignalHandlerLoop(logging.Global(), stopCancel{cancel: cancel, stackDriverSender: sds})
	s.Run(ctx)
}
