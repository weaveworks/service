// This module is a CLI tool to interact with the Google Kubernetes Engine API.
// Usage:
// - go to service-conf/k8s/<env>/default/gcp-launcher-secret.yaml
// - copy the value for "cloud-launcher.json"
// - run: $ echo -n "<value copied>" | base64 --decode > ~/.ssh/cloud-launcher-<env>.json
// - run: $ go run common/gcp/gke/cmd/main.go -service-account-key-file=~/.ssh/cloud-launcher-<env>.json
//
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/weaveworks/service/common/gcp/gke"
)

// Config for the Kubernetes Engine API
type Config struct {
	ServiceAccountKeyFile string // We need an OAuth2 client
	ProjectID             string
	Zone                  string
	RunKubectlCmd         bool
}

// RegisterFlags sets up config for the Partner Subscriptions API.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	flag.StringVar(&cfg.ServiceAccountKeyFile, "service-account-key-file", "", "Service account key JSON file")
	flag.StringVar(&cfg.ProjectID, "project-id", "gke-integration", "GCP project's ID")
	flag.StringVar(&cfg.Zone, "zone", "us-central1-a", "GKE cluster's zone")
	flag.BoolVar(&cfg.RunKubectlCmd, "run-kubectl-cmd", false, "Run an apply kubectl command if set to true. Default: false")
}

const prometheusPort = 9000

func main() {
	var cfg gke.Config
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	// Expose Prometheus metrics:
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%v", prometheusPort), nil)

	gkeClient, err := gke.NewClientFromConfig(cfg.ServiceAccountKeyFile)
	if err != nil {
		log.Fatalf("Failed creating Google Kubernetes Engine API client: %v", err)
	}

	projects, err := gkeClient.ListProjects(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, project := range projects {
		log.Printf("GCP project: %v\n", project.Name)
	}
	log.Println("----")

	clusters, err := gkeClient.ListClusters(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, cluster := range clusters {
		log.Printf("GKE cluster: %v\n", cluster.Name)
	}
	log.Println("----")

	// The Google Developers Console [project ID or project
	// number](https://support.google.com/cloud/answer/6158840).
	projectID := cfg.ProjectID

	clusters, err = gkeClient.ListClustersForProject(context.Background(), projectID)
	if err != nil {
		log.Fatal(err)
	}
	for _, cluster := range clusters {
		log.Printf("GKE cluster: %v\n", cluster.Name)
	}
	log.Println("----")

	// The name of the Google Compute Engine
	// [zone](/compute/docs/zones#available) in which the cluster
	// resides, or "-" for all zones.
	zone := cfg.Zone

	resp, err := gkeClient.ListClustersForProjectAndZone(context.Background(), projectID, zone)
	if err != nil {
		log.Fatal(err)
	}

	for _, cluster := range resp.Clusters {
		log.Printf("%#v\n", cluster.Name)
		kubeCfg := gke.NewKubeConfig(
			cluster.Name,
			cluster.Endpoint,
			cluster.MasterAuth.Username,
			cluster.MasterAuth.Password,
			cluster.MasterAuth.ClusterCaCertificate)
		kubeCfgFile, err := writeToFile(kubeCfg)
		if err != nil {
			log.Fatalf("Failed to marshal YAML for kubeconfig: %v", err)
		}
		log.Printf("%#v\n", kubeCfg)
		defer os.Remove(kubeCfgFile.Name())
		runCmd(fmt.Sprintf("kubectl --kubeconfig=%v get pods --all-namespaces", kubeCfgFile.Name()))
		runCmd(fmt.Sprintf("kubectl --kubeconfig=%v apply -n kube-system -f \"https://cloud.weave.works/k8s.yaml?k8s-version=%v&t=5axdfb9p17oeoouho8neupd4db8my8tb\"", kubeCfgFile.Name(), cluster.CurrentMasterVersion))
		runCmd(fmt.Sprintf("kubectl --kubeconfig=%v get pods --all-namespaces", kubeCfgFile.Name()))
	}

	curl := exec.Command("curl", "-fsS", fmt.Sprintf("localhost:%v/metrics", prometheusPort))
	grep := exec.Command("grep", "google_gke_client_request_duration_seconds_sum")
	r, w := io.Pipe()
	curl.Stdout = w
	grep.Stdin = r
	var buffer bytes.Buffer
	grep.Stdout = &buffer
	curl.Start()
	grep.Start()
	curl.Wait()
	w.Close()
	grep.Wait()
	io.Copy(os.Stdout, &buffer)
	log.Printf("%v\n", string(buffer.Bytes()))

	if *cfg.RunKubectlCmd {
		createResp, err := gkeClient.CreateCluster(context.Background(), projectID, zone)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%#v\n", createResp)
	}
}

func runCmd(cmdLine string) {
	cmdAndArgs := strings.Split(cmdLine, " ")
	cmd := exec.Command(cmdAndArgs[0], cmdAndArgs[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Command %v failed: %v", cmd.Args, err)
	}
	log.Printf("%v\n", string(output))
}

func writeToFile(kubeCfg *gke.KubeConfig) (*os.File, error) {
	content, err := kubeCfg.Marshal()
	if err != nil {
		return nil, err
	}
	tmpfile, err := ioutil.TempFile("/tmp", "kubeconfig")
	if err != nil {
		return nil, err
	}
	if _, err := tmpfile.Write(content); err != nil {
		return nil, err
	}
	if err := tmpfile.Close(); err != nil {
		return nil, err
	}
	return tmpfile, nil
}
