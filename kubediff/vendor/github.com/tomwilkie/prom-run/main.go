package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	statusCode = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "command_exit_code",
		Help: "Exit code of command.",
	})
	commandDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "command_duration_seconds",
		Help: "Time spent running command.",
	})
)

func main() {
	var (
		period     = flag.Duration("period", 10*time.Second, "Period with which to run the command.")
		listenAddr = flag.String("listen-addr", ":9152", "Address to listen on")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s (options) command...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if len(flag.Args()) <= 0 {
		flag.Usage()
		os.Exit(2)
	}
	command := flag.Args()[0]
	args := flag.Args()[1:]

	var (
		outputLock      sync.Mutex
		outputBuf       []byte
		lastRunStart    time.Time
		lastRunDuration time.Duration
		withLock        = func(f func()) {
			outputLock.Lock()
			defer outputLock.Unlock()
			f()
		}
	)

	go func() {
		for range time.Tick(*period) {
			log.Printf("Running '%s' with argments %v", command, args)
			start := time.Now()
			out, err := exec.Command(command, args...).CombinedOutput()
			duration := time.Now().Sub(start)
			commandDuration.Observe(duration.Seconds())

			withLock(func() {
				lastRunStart = start
				lastRunDuration = duration
				outputBuf = out
			})

			if err != nil {
				if exiterr, ok := err.(*exec.ExitError); ok {
					if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
						code := status.ExitStatus()
						log.Printf("Command exited with code: %d", code)
						statusCode.Set(float64(code))
						continue
					}
				}
				log.Printf("Error running command: %v", err)
				statusCode.Set(255)
				continue
			}

			log.Printf("Command exited successfully")
			statusCode.Set(0)
		}
	}()

	tmpl, err := template.New("index").Parse(`<html>
	<head><title>Prometheus Command Runner</title></head>
	<body>
	  <h2>Prometheus Command Runner</h2>
	  <p>"{{.Command}}" output:</p>
	  <pre>{{.Output}}</pre>
	  <p>Run at {{.Time}} took {{.Duration}}<p>
	</body>
	</html>`)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	prometheus.MustRegister(statusCode)
	prometheus.MustRegister(commandDuration)
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		withLock(func() {
			tmpl.Execute(w, struct {
				Command, Output string
				Time            time.Time
				Duration        time.Duration
			}{
				Command:  command + " " + strings.Join(args, " "),
				Output:   string(outputBuf),
				Time:     lastRunStart,
				Duration: lastRunDuration,
			})
		})
	}))
	http.Handle("/metrics", prometheus.Handler())

	log.Printf("Listening on address %s", *listenAddr)
	http.ListenAndServe(*listenAddr, nil)
}
