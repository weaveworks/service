package main

import (
	"flag"

	"google.golang.org/grpc"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/cortex/util"
	"github.com/weaveworks/service/prom/db"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace: "prom",
			// XXX: Cargo-culted from distributor. Probably don't need this
			// for configs just yet?
			GRPCMiddleware: []grpc.UnaryServerInterceptor{
				middleware.ServerUserHeaderInterceptor,
			},
		}
		dbConfig DynamoDBConfig
	)
	util.RegisterFlags(&serverConfig, &dbConfig)
	flag.Parse()

	db, err := db.MustNew()
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	a := NewAPI(DynamoDBClient{db})

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initializing server: %v", err)
	}
	defer server.Shutdown()

	a.RegisterRoutes(server.HTTP)
	server.Run()
}
