package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/server"
	"github.com/weaveworks/service/service-net-discovery/peers"
)

func main() {
	var (
		serverConfig = server.Config{
			MetricsNamespace: peers.MetricsNamespace,
		}
		peersConfig peers.PeerDiscoveryConfig
	)
	serverConfig.RegisterFlags(flag.CommandLine)
	flag.StringVar(&peersConfig.DynamodbURL, "app.dynamodb.url", "", "dynamodb url")
	flag.Parse()

	s, err := server.New(serverConfig)
	if err != nil {
		log.Fatal(err)
	}

	discovery, err := peers.New(peersConfig)
	if err != nil {
		log.Fatal(err)
	}
	s.HTTP.Methods("GET").Path("/api/net/peer").HandlerFunc(discovery.ListPeers)
	s.HTTP.Methods("POST").Path("/api/net/peer").HandlerFunc(discovery.UpdatePeer)
	s.HTTP.Methods("DELETE").Path("/api/net/peer").HandlerFunc(discovery.DeletePeer)

	defer log.Info("app exiting")

	log.Info("listening for requests on port", serverConfig.HTTPListenPort)
	s.Run()

	// TODO: any shutdown steps
}
