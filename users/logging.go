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

func (w *apacheLoggingResponseWriter) Write(b []byte) (int, error) {
	if !w.Started {
		w.Started = true
		w.StatusCode = http.StatusOK
	}

	n, err := w.ResponseWriter.Write(b)
	w.BytesWritten += int64(n)
	return n, err
}

func (w *apacheLoggingResponseWriter) WriteHeader(status int) {
	if !w.Started {
		w.Started = true
		w.StatusCode = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *apacheLoggingResponseWriter) String() string {
	// TODO: Handle X-ForwardedFor
	clientIP := w.r.RemoteAddr
	if colon := strings.LastIndex(clientIP, ":"); colon != -1 {
		clientIP = clientIP[:colon]
	}

	return fmt.Sprintf(
		"%s - - [%s] \"%s %d %d\" %f",
		clientIP,
		w.finishedAt.Format("02/Jan/2006 03:04:05"),
		fmt.Sprintf("%s %s %s", w.r.Method, w.r.RequestURI, w.r.Proto),
		w.StatusCode,
		w.BytesWritten,
		w.finishedAt.Sub(w.startedAt).Seconds(),
	)
}
