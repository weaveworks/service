package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/prom/api"
	"github.com/weaveworks/service/prom/db"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace: "prom",
		}
		dbConfig db.Config
	)

	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine)

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	a := api.New(db)

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initializing server: %v", err)
	}
	defer server.Shutdown()

	a.RegisterRoutes(server.HTTP)
	server.Run()
}
