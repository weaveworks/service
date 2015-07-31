package main

import (
	"net/http"

	"github.com/Sirupsen/logrus"
)

func main() {

	flags := parseFlags()
	setupLogging(flags.logLevel)

	authenticator := getAuthenticator(flags)
	orgMapper := getOrganizationMapper(flags)
	appProxyHandle := func(w http.ResponseWriter, r *http.Request) {
		appProxy(authenticator, orgMapper, w, r)
	}
	http.HandleFunc("/", appProxyHandle)
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

func getAuthenticator(f *flags) authenticator {
	if f.authenticatorType == "mock" {
		return &mockAuthenticator{}
	}

	return &webAuthenticator{
		serverHost: f.authenticatorHost,
	}
}

func getOrganizationMapper(f *flags) organizationMapper {
	if f.mapperType == "constant" {
		return &constantMapper{
			targetHost: f.constantMapperTargetHost,
		}
	}

	return &dbMapper{
		dbHost: f.appMapperDBHost,
	}
}
