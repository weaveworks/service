package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/weaveworks/service/dashboard-api/dashboard"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] dashboard-id\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	namespace := flag.String("namespace", "default", "workload namespace")
	workload := flag.String("workload", "", "workload name")
	rangeSelector := flag.String("range", "2m", "selector for range vectors")
	js := flag.Bool("js", false, "output a js variable with the resulting JSON")

	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(1)
	}
	if *workload == "" {
		fmt.Fprintf(os.Stderr, "error: a workload name needs to be specified with -workload\n")
		os.Exit(1)
	}

	ID := flag.Arg(0)

	if err := dashboard.Init(); err != nil {
		log.Fatalf("error initializing dashboards: %v", err)
	}

	d, err := dashboard.GetDashboardByID(ID, map[string]string{
		"namespace": *namespace,
		"workload":  *workload,
		"range":     *rangeSelector,
	})
	if err != nil {
		log.Fatalf("error getting dashboard by ID: %v", err)
	}

	if d == nil {
		fmt.Fprintf(os.Stderr, "error: couldn't find dashboard '%s'\n", ID)
		os.Exit(1)
	}

	if *js {
		fmt.Fprintf(os.Stdout, "export const DashboardsJSON = `[")
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(d)
	if *js {
		fmt.Fprintf(os.Stdout, "]`;\n")
	}
}
