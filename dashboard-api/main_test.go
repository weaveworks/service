package main

import (
	"flag"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/logging"
)

var update = flag.Bool("update", false, "update .golden files")
var logLevel = flag.String("log.level", "debug", "the log level")

func TestMain(m *testing.M) {
	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("error initializing logging: %v", err)
	}

	flag.Parse()
	os.Exit(m.Run())
}
