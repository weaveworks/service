package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/notebooks/api"
	"github.com/weaveworks/service/notebooks/db"
	users "github.com/weaveworks/service/users/client"
)

func main() {
	var (
		cfg          Config
		serverConfig = server.Config{
			MetricsNamespace: "notebooks",
		}
		dbConfig db.Config
	)

	cfg.RegisterFlags(flag.CommandLine)
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

	authOptions := users.CachingClientConfig{}
	usersClient, err := users.New(cfg.usersServiceType, cfg.usersServiceURL, authOptions)
	if err != nil {
		log.Fatalf("Error making users client: %v", err)
		return
	}

	a := api.New(db, usersClient)
	a.RegisterRoutes(server.HTTP)
	server.Run()
}
