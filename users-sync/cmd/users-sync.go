package main

import (
	"context"
	"flag"

	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/users/marketing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	commonServer "github.com/weaveworks/common/server"
	"github.com/weaveworks/common/tracing"
	"google.golang.org/grpc"

	"github.com/weaveworks/service/common"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users-sync/attrsync"
	"github.com/weaveworks/service/users-sync/cleaner"
	"github.com/weaveworks/service/users-sync/server"
	"github.com/weaveworks/service/users-sync/weeklyreporter"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/render"
)

func init() {
	prometheus.MustRegister(common.DatabaseRequestDuration)
}

func main() {
	traceCloser := tracing.NewFromEnv("users-sync")
	defer traceCloser.Close()

	var (
		segementWriteKeyFile = flag.String("segment-write-key-file", "", "File containing segment write key")

		dbCfg      dbconfig.Config
		billingCfg billing_grpc.Config
		marketoCfg marketing.MarketoConfig
		usersCfg   users.Config

		serverConfig = commonServer.Config{
			MetricsNamespace: common.PrometheusNamespace,
			GRPCMiddleware:   []grpc.UnaryServerInterceptor{render.GRPCErrorInterceptor},
		}

		cleanupURLs common.ArrayFlags
	)

	flag.Var(&cleanupURLs, "cleanup-url", "Endpoints for cleanup after instance deletion")
	dbCfg.RegisterFlags(flag.CommandLine, "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)", "", "Migrations directory.")
	billingCfg.RegisterFlags(flag.CommandLine)
	marketoCfg.RegisterFlags(flag.CommandLine)
	usersCfg.RegisterFlags(flag.CommandLine)
	serverConfig.RegisterFlags(flag.CommandLine)
	flag.CommandLine.IntVar(&serverConfig.HTTPListenPort, "port", 80, "HTTP port to listen on")
	flag.CommandLine.IntVar(&serverConfig.GRPCListenPort, "grpc-port", 4772, "gRPC port to listen on")

	flag.Parse()

	if err := logging.Setup(serverConfig.LogLevel.String()); err != nil {
		logrus.Fatalf("Error configuring logging: %v", err)
		return
	}

	db := db.MustNew(dbCfg)
	defer db.Close(context.Background())

	logger := logging.Logrus(logrus.StandardLogger())

	usersClient, err := users.NewClient(usersCfg)
	if err != nil {
		logrus.Fatalf("Failed creating users client: %v", err)
	}

	billingClient, err := billing_grpc.NewClient(billingCfg)
	if err != nil {
		logrus.Fatalf("Failed creating billing-api's gRPC client: %v", err)
	}
	defer billingClient.Close()

	segmentClient, err := attrsync.NewSegmentClient(*segementWriteKeyFile, logger)
	if err != nil {
		logrus.Fatalf("Failed creating a segment client: %v", err)
	}
	defer segmentClient.Close()

	marketoClient, err := attrsync.NewMarketoClient(marketoCfg)
	if err != nil {
		logrus.Fatalf("Failed creating a marketo client: %v", err)
	}

	weeklyReporter := weeklyreporter.New(logger, usersClient)
	orgCleaner := cleaner.New(cleanupURLs, logger, db)
	attributeSyncer := attrsync.New(
		logger, db, billingClient, segmentClient, marketoClient)
	logger.Debugln("Debug logging enabled")

	logger.Infof("users-sync listening on ports %d (HTTP) and %d (gRPC)", serverConfig.HTTPListenPort, serverConfig.GRPCListenPort)
	cServer, err := commonServer.New(serverConfig)
	if err != nil {
		logrus.Fatalf("Failed to create server: %v", err)
		return
	}
	userSyncServer := server.New(logger, orgCleaner, attributeSyncer, weeklyReporter)
	api.RegisterUsersSyncServer(cServer.GRPC, userSyncServer)

	weeklyReporter.Start()
	defer weeklyReporter.Stop()
	orgCleaner.Start()
	defer orgCleaner.Stop()
	attributeSyncer.Start()
	defer attributeSyncer.Stop()

	defer cServer.Shutdown()
	cServer.Run()
}
