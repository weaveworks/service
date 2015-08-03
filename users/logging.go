package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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

func loggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrappedWriter := &apacheLoggingResponseWriter{
			ResponseWriter: w,
			r:              r,
			startedAt:      time.Now().UTC(),
		}
		handler.ServeHTTP(wrappedWriter, r)
		wrappedWriter.finishedAt = time.Now().UTC()

		logrus.Info(wrappedWriter.String())
	})
}

type apacheLoggingResponseWriter struct {
	http.ResponseWriter
	r            *http.Request
	Started      bool
	StatusCode   int
	BytesWritten int64

	startedAt  time.Time
	finishedAt time.Time
}

func (this *apacheLoggingResponseWriter) Write(b []byte) (int, error) {
	if !this.Started {
		this.Started = true
		this.StatusCode = http.StatusOK
	}

	n, err := this.ResponseWriter.Write(b)
	this.BytesWritten += int64(n)
	return n, err
}

func (this *apacheLoggingResponseWriter) WriteHeader(status int) {
	if !this.Started {
		this.Started = true
		this.StatusCode = status
	}
	this.ResponseWriter.WriteHeader(status)
}

func (this *apacheLoggingResponseWriter) String() string {
	// TODO: Handle X-ForwardedFor
	clientIP := this.r.RemoteAddr
	if colon := strings.LastIndex(clientIP, ":"); colon != -1 {
		clientIP = clientIP[:colon]
	}

	return fmt.Sprintf(
		"%s - - [%s] \"%s %d %d\" %f",
		clientIP,
		this.finishedAt.Format("02/Jan/2006 03:04:05"),
		fmt.Sprintf("%s %s %s", this.r.Method, this.r.RequestURI, this.r.Proto),
		this.StatusCode,
		this.BytesWritten,
		this.finishedAt.Sub(this.startedAt).Seconds(),
	)
}
