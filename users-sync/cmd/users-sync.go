package main

import (
	"context"
	"flag"
	"io/ioutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/segmentio/analytics-go"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	commonServer "github.com/weaveworks/common/server"
	"github.com/weaveworks/common/tracing"
	"google.golang.org/grpc"

	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users-sync/attrsync"
	"github.com/weaveworks/service/users-sync/cleaner"
	"github.com/weaveworks/service/users-sync/server"
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
		logLevel             = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
		port                 = flag.Int("port", 80, "port to listen on")
		grpcPort             = flag.Int("grpc-port", 4772, "grpc port to listen on")
		segementWriteKeyFile = flag.String("segment-write-key-file", "", "File containing segment write key")

		dbCfg dbconfig.Config

		cleanupURLs common.ArrayFlags
	)

	flag.Var(&cleanupURLs, "cleanup-url", "Endpoints for cleanup after instance deletion")
	dbCfg.RegisterFlags(flag.CommandLine, "postgres://postgres@users-db.weave.local/users?sslmode=disable", "URI where the database can be found (for dev you can use memory://)", "/migrations", "Migrations directory.")

	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	db := db.MustNew(dbCfg)
	defer db.Close(context.Background())

	logger := logging.Logrus(log.StandardLogger())

	var segmentClient analytics.Client
	if segementWriteKeyFile != nil {
		segementWriteKeyBytes, err := ioutil.ReadFile(*segementWriteKeyFile)
		if err != nil {
			log.Fatalln("Failed to read segment write key", err)
			return
		}
		segmentClient := analytics.New(string(segementWriteKeyBytes))
		defer segmentClient.Close()
	}

	orgCleaner := cleaner.New(cleanupURLs, logger, db)
	attributeSyncer := attrsync.New(logger, db, nil, segmentClient)
	log.Debug("Debug logging enabled")

	log.Infof("Listening on port %d", *port)
	cServer, err := commonServer.New(commonServer.Config{
		MetricsNamespace:        common.PrometheusNamespace,
		HTTPListenPort:          *port,
		GRPCListenPort:          *grpcPort,
		GRPCMiddleware:          []grpc.UnaryServerInterceptor{render.GRPCErrorInterceptor},
		RegisterInstrumentation: true,
		Log:                     logging.Logrus(log.StandardLogger()),
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
		return
	}
	userSyncServer := server.New(logger)
	api.RegisterUsersSyncServer(cServer.GRPC, userSyncServer)

	orgCleaner.Start()
	defer orgCleaner.Stop()
	attributeSyncer.Start()
	defer attributeSyncer.Stop()

	defer cServer.Shutdown()
	cServer.Run()
}
