package main

import (
	"context"
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"

	"github.com/weaveworks/service/billing-api/db"
	"github.com/weaveworks/service/billing-api/grpc"
	"github.com/weaveworks/service/billing-api/routes"
	common_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/dbconfig"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/common/zuora"
)

// Config holds the API settings.
type Config struct {
	dbConfig     dbconfig.Config
	routesConfig routes.Config
	serverConfig server.Config
	usersConfig  users.Config
	zuoraConfig  zuora.Config
}

// RegisterFlags registers configuration variables.
func (c *Config) RegisterFlags(f *flag.FlagSet) {
	c.dbConfig.RegisterFlags(f, "postgres://postgres@billing-db/billing?sslmode=disable", "Database to use.", "/migrations", "Migrations directory.")
	c.routesConfig.RegisterFlags(f)
	c.serverConfig.RegisterFlags(f)
	c.usersConfig.RegisterFlags(f)
	c.zuoraConfig.RegisterFlags(f)
}

// Validate calls validation on its sub configs.
func (c *Config) Validate() error {
	return c.zuoraConfig.Validate(true)
}

func main() {
	cfg := Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()
	cfg.serverConfig.MetricsNamespace = "billing"

	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	if err := logging.Setup(cfg.serverConfig.LogLevel.String()); err != nil {
		log.Fatalf("error initialising logging: %v", err)
	}
	cfg.serverConfig.Log = logging.Logrus(log.StandardLogger())

	users, err := users.NewClient(cfg.usersConfig)
	if err != nil {
		log.Fatalf("error initialising users client: %v", err)
	}

	z := zuora.New(cfg.zuoraConfig, nil)

	db, err := db.New(cfg.dbConfig)
	if err != nil {
		log.Fatalf("error initialising database client: %v", err)
	}
	defer db.Close(context.Background())

	server, err := server.New(cfg.serverConfig)
	if err != nil {
		log.Fatalf("error initialising server: %v", err)
	}
	defer server.Shutdown()

	routes, err := routes.New(cfg.routesConfig, db, users, z)
	if err != nil {
		log.Fatalf("error initialising api: %v", err)
	}
	routes.RegisterRoutes(server.HTTP)
	log.WithField("port", cfg.serverConfig.HTTPListenPort).Infof("billing-api now serving HTTP requests")
	common_grpc.RegisterBillingServer(server.GRPC, grpc.Server{DB: db})
	log.WithField("port", cfg.serverConfig.GRPCListenPort).Infof("billing-api now serving gRPC requests")
	server.Run()
}
