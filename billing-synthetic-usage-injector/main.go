package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/signals"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector"
	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/config"
)

func main() {
	config := config.Flags{}
	config.RegisterFlags(flag.CommandLine)
	flag.Parse()

	injector, err := injector.NewSyntheticUsageInjector(&config)
	if err != nil {
		log.WithField("err", err).Fatal("failed to create synthetic usage injector")
	}
	injector.Start()

	signals.SignalHandlerLoop(
		logging.Logrus(log.StandardLogger()),
		injector,
	)
}
