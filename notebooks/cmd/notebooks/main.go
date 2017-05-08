package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/notebooks/api"
	"github.com/weaveworks/service/notebooks/db"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace: "notebooks",
		}
		dbConfig db.Config
	)

	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initializing server: %v", err)
	}
	defer server.Shutdown()

	a := api.New(db)
	a.RegisterRoutes(server.HTTP)
	server.Run()
}
