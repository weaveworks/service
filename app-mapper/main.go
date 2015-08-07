package main

import (
	"net/http"

	"github.com/Sirupsen/logrus"
)

func main() {

	flags := parseFlags()
	setupLogging(flags.logLevel)

	authenticator := flags.getAuthenticator()
	appProvisioner := flags.getAppProvisioner()
	logrus.Info("Fetching application")
	err := appProvisioner.fetchApp()
	if err != nil {
		logrus.Error("Couldn't fetch application: ", err)
	}
	orgMapper := flags.getOrganizationMapper(appProvisioner)
	http.Handle("/", makeProxyHandler(authenticator, orgMapper))
	logrus.Info("Listening on :80")
	handler := makeLoggingHandler(http.DefaultServeMux)
	logrus.Fatal(http.ListenAndServe(":80", handler))
}

func makeLoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
