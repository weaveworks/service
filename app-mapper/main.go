package main

import (
	"flag"
	"net/http"

	"github.com/Sirupsen/logrus"
)

var (
	configFile string
)

func main() {
	flag.StringVar(&configFile, "config-file", "./config.yaml", "Path to configuration file")
	flag.Parse()

	config, err := parseConfig(configFile)
	if err != nil {
		logrus.Fatal(err)
	}
	setupLogging(config.logLevel)

	authenticator := getAuthenticator(config)
	orgMapper := getOrganizationMapper(config)
	appProxyHandle := func(w http.ResponseWriter, r *http.Request) {
		appProxy(authenticator, orgMapper, w, r)
	}
	http.HandleFunc("/app/", appProxyHandle)
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

func getAuthenticator(c *config) authenticator {
	if c.authenticatorType == "mock" {
		return &mockAuthenticator{}
	}

	return &webAuthenticator{
		ServerHost: c.authenticatorHost,
	}
}

func getOrganizationMapper(c *config) organizationMapper {
	if c.mapperType == "constant" {
		return &constantMapper{
			targetHost: c.constantMapperTargetHost,
		}
	}

	return &dbMapper{
		dbHost: c.appMapperDBHost,
	}
}
