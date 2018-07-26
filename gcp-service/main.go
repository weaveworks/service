package main

import (
	"flag"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/weaveworks/common/logging"
	commonserver "github.com/weaveworks/common/server"
	"github.com/weaveworks/service/common"
	"github.com/weaveworks/service/common/gcp/gke"
	"github.com/weaveworks/service/common/users"

	"github.com/weaveworks/service/gcp-service/dao"
	"github.com/weaveworks/service/gcp-service/grpc"
	"github.com/weaveworks/service/gcp-service/http"
	"github.com/weaveworks/service/gcp-service/render"
	"github.com/weaveworks/service/gcp-service/service"
	kubectl "github.com/weaveworks/service/kubectl-service/grpc"
	googlegrpc "google.golang.org/grpc"
)

func main() {
	var (
		logLevel      = flag.String("log.level", "info", "Logging level to use: debug | info | warn | error")
		httpPort      = flag.Int("port", 80, "HTTP port to listen on")
		grpcPort      = flag.Int("grpc-port", 4772, "gRpc port to listen on")
		dryRun        = flag.Bool("dry-run", false, "Do NOT actually run DAO calls, but mock them and return arbitrary values.")
		usersConfig   users.Config
		kubectlConfig kubectl.Config
	)
	usersConfig.RegisterFlags(flag.CommandLine)
	kubectlConfig.RegisterFlags(flag.CommandLine)
	flag.Parse()

	if err := logging.Setup(*logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
		return
	}

	log.Infof("gcp-service configured to listen on ports %d (HTTP) and %d (gRPC)", *httpPort, *grpcPort)
	serv, err := commonserver.New(commonserver.Config{
		MetricsNamespace:        common.PrometheusNamespace,
		HTTPListenPort:          *httpPort,
		GRPCListenPort:          *grpcPort,
		GRPCMiddleware:          []googlegrpc.UnaryServerInterceptor{render.GRPCErrorInterceptor},
		RegisterInstrumentation: true,
		Log: logging.Logrus(log.StandardLogger()),
	})
	if err != nil {
		log.Fatalf("Failed to create gcp-service's server: %v", err)
	}
	defer serv.Shutdown()

	kubectlClient, err := newKubectlClient(*dryRun, kubectlConfig)
	if err != nil {
		log.Fatalf("Failed to create client for kubectl-service: %v", err)
	}
	defer kubectlClient.Close()

	svc := &service.Service{
		UsersClient:      newUsersClient(*dryRun, usersConfig),
		KubectlClient:    kubectlClient,
		GKEClientFactory: newGKEClientFactory(*dryRun),
	}

	hserv := &http.Server{Service: svc}
	hserv.RegisterRoutes(serv.HTTP)

	gserv := &grpc.Server{Service: svc}
	grpc.RegisterGCPServer(serv.GRPC, gserv)

	log.Infof("gcp-service now running...")
	serv.Run()
}

func newKubectlClient(dryRun bool, kubectlConfig kubectl.Config) (kubectl.CloseableKubectlClient, error) {
	if dryRun {
		log.Infof("Dry run mode activated: no call will actually be made to the kubectl-service.")
		return &kubectl.NoOpClient{}, nil
	}
	return kubectl.NewClient(kubectlConfig)
}

func newUsersClient(dryRun bool, usersConfig users.Config) dao.UsersClient {
	if dryRun {
		log.Infof("Dry run mode activated: no call will actually be made to the users service.")
		return &dao.UsersNoOpClient{}
	}
	return &dao.UsersHTTPClient{UsersHostPort: usersConfig.HostPort}
}

func newGKEClientFactory(dryRun bool) func(*oauth2.Token) (gke.Client, error) {
	if dryRun {
		return func(token *oauth2.Token) (gke.Client, error) {
			return gke.NoOpClient{}, nil
		}
	}
	return func(token *oauth2.Token) (gke.Client, error) {
		return gke.NewClientFromToken(token)
	}
}
