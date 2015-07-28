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

	config, err := ParseConfig(configFile)
	if err != nil {
		logrus.Fatal(err)
	}
	setupLogging((*config).LogLevel)

	authenticator := getAuthenticator(config)
	orgMapper := getOrganizationMapper(config)
	appProxyHandle := func(w http.ResponseWriter, r *http.Request) {
		AppProxy(authenticator, orgMapper, w, r)
	}
	http.HandleFunc("/app/", appProxyHandle)
	logrus.Info("Listening on :80")
	handler := makeLoggingHandler(http.DefaultServeMux)
	logrus.Fatal(http.ListenAndServe(":80", handler))
}

func makeLoggingHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logrus.Infof("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func getAuthenticator(c *Config) Authenticator {
	if c.AuthenticatorType == "mock" {
		return &MockAuthenticator{}
	} else {
		return &WebAuthenticator{
			ServerHost: c.AuthenticatorHost,
		}
	}
}

func getOrganizationMapper(c *Config) OrganizationMapper {
	if c.MapperType == "constant" {
		return &ConstantMapper{
			TargetHost: c.ConstantMapperTargetHost,
		}
	} else {
		return &DBMapper{
			DBHost: c.AppMapperDBHost,
		}
	}
}
