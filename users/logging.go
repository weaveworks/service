package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

func setupLogging(logLevel string) {
	logrus.SetOutput(os.Stderr)

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.SetLevel(level)
}

func instrument(m routeMatcher, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrappedWriter := &apacheLoggingResponseWriter{
			ResponseWriter: w,
			r:              r,
			startedAt:      time.Now().UTC(),
		}

		next.ServeHTTP(wrappedWriter, r)

		wrappedWriter.finishedAt = time.Now().UTC()
		logrus.Info(wrappedWriter.String())
		var (
			method = r.Method
			route  = normalizePath(m, r)
			status = strconv.Itoa(wrappedWriter.StatusCode)
			took   = wrappedWriter.finishedAt.Sub(wrappedWriter.startedAt)
		)
		logrus.Debugf("%s: %s %s (%s) %s", r.URL.Path, method, route, status, took)
		requestLatency.WithLabelValues(method, route, status).Observe(float64(took.Nanoseconds()))
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

type routeMatcher interface {
	Match(*http.Request, *mux.RouteMatch) bool
}

func normalizePath(m routeMatcher, r *http.Request) string {
	var match mux.RouteMatch
	if !m.Match(r, &match) {
		logrus.Warnf("couldn't normalize path: %s", r.URL.Path)
		return "unmatched_path"
	}
	name := match.Route.GetName()
	if name == "" {
		logrus.Warnf("path isn't named: %s", r.URL.Path)
		return "unnamed_path"
	}
	return name
}
