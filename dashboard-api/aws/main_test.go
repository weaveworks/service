package aws

import (
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "update .golden files")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
