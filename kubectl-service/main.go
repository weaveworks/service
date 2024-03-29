package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	commonserver "github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"

	"github.com/weaveworks/service/kubectl-service/grpc"
	"github.com/weaveworks/service/kubectl-service/render"
	googlegrpc "google.golang.org/grpc"
)

func main() {
	serverConfig := commonserver.Config{
		MetricsNamespace: common.PrometheusNamespace,
		GRPCMiddleware:   []googlegrpc.UnaryServerInterceptor{render.GRPCErrorInterceptor},
	}
	serverConfig.RegisterFlags(flag.CommandLine)
	flag.CommandLine.IntVar(&serverConfig.HTTPListenPort, "port", 80, "HTTP port to listen on")
	flag.CommandLine.IntVar(&serverConfig.GRPCListenPort, "grpc-port", 4772, "gRPC port to listen on")
	var (
		dryRun = flag.Bool("dry-run", false, "Do NOT actually run kubectl, but simply log the command.")
	)
	flag.Parse()

	log.Infof("kubectl-service configured to listen on ports %d (HTTP) and %d (gRPC)", serverConfig.HTTPListenPort, serverConfig.GRPCListenPort)
	serv, err := commonserver.New(serverConfig)
	if err != nil {
		log.Fatalf("Failed to create kubectl-service: %v", err)
	}
	defer serv.Shutdown()

	gserv, err := grpc.NewServer(newRunner(*dryRun))
	if err != nil {
		log.Fatalf("Failed to create kubectl-service's gRPC server: %v", err)
	}

	grpc.RegisterKubectlServer(serv.GRPC, gserv)
	log.Infof("kubectl-service now running...")
	serv.Run()
}

func newRunner(dryRun bool) grpc.KubectlRunner {
	if dryRun {
		log.Infof("Dry run mode activated: no kubectl command will actually be run.")
		return &grpc.NoOpKubectlRunner{}
	}
	return &grpc.DefaultKubectlRunner{}
}
