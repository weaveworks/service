package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/notification-configmanager"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace: "notification",
		}
		configManagerConfig configmanager.Config
		logLevel            string
	)
	serverConfig.RegisterFlags(flag.CommandLine)
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	configManagerConfig.RegisterFlags()
	flag.Parse()

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	configManager, err := configmanager.New(configManagerConfig)
	if err != nil {
		log.Fatal(err)
	}

	s, err := server.New(serverConfig)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("listening for requests")
	configManager.Register(s.HTTP)

	defer log.Info("app exiting")
	s.Run()
}
