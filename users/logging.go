package main

import (
	"os"

	"github.com/Sirupsen/logrus"
)

func setupLogging(logLevel string) {
	logrus.SetOutput(os.Stderr)

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.SetLevel(level)
}
