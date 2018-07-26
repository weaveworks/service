package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common/dbconfig"
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
		dbConfig dbconfig.Config
	)

	cfg.RegisterFlags(flag.CommandLine)
	serverConfig.RegisterFlags(flag.CommandLine)
	dbConfig.RegisterFlags(flag.CommandLine,
		"postgres://postgres@configs-db.default.svc.cluster.local/notebooks?sslmode=disable",
		"URI where the database can be found (for dev you can use memory://)",
		"",
		"Path where the database migration files can be found")

	flag.Parse()

	if err := logging.Setup(serverConfig.LogLevel.String()); err != nil {
		log.Fatalf("error initialising logging: %v", err)
	}
	serverConfig.Log = logging.Logrus(log.StandardLogger())

	db, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	server, err := server.New(serverConfig)
	if err != nil {
		log.Fatalf("Error initializing server: %v", err)
	}
	defer server.Shutdown()

	usersOptions := users.CachingClientConfig{}
	if cfg.usersCacheSize > 0 {
		usersOptions.CacheEnabled = true
		usersOptions.OrgCredCacheSize = cfg.usersCacheSize
		usersOptions.ProbeCredCacheSize = cfg.usersCacheSize
		usersOptions.UserCacheSize = cfg.usersCacheSize
		usersOptions.OrgCredCacheExpiration = cfg.usersCacheExpiration
		usersOptions.ProbeCredCacheExpiration = cfg.usersCacheExpiration
		usersOptions.UserCacheExpiration = cfg.usersCacheExpiration
	}
	usersClient, err := users.New(cfg.usersServiceType, cfg.usersServiceURL, usersOptions)
	if err != nil {
		log.Fatalf("Error making users client: %v", err)
		return
	}

	a := api.New(db, usersClient)
	a.RegisterRoutes(server.HTTP)
	server.Run()
}
