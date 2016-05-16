package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"time"
)

func main() {
	var (
		clusterName = flag.String("cluster-name", "", "Name of the Kubernetes cluster to connect to, e.g. \"kubernetes-devz\", \"kubernetes-prod1\"")
		ec2Region   = flag.String("ec2-region", "", "Name of EC2 region cluster is in, e.g. \"us-east-1\"")
		kubeConfig  = flag.String("kubeconfig", "", "Path to Kubernetes configuration file, e.g. \"infra/dev/kubeconfig\"")
		sshKey      = flag.String("ssh-key-path", "", "Path to SSH key, e.g. \"infra/dev/ec2_weaveworks.kubernetes-anywhere.us-east-1.pem\"")
	)

	flag.Parse()

	if *clusterName == "" || *ec2Region == "" || *kubeConfig == "" || *sshKey == "" {
		flag.Usage()
		os.Exit(1)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	client := &kubeClient{ConfigPath: *kubeConfig, UserName: os.Getenv("USER")}

	nodes, err := getEC2Nodes(*ec2Region, *clusterName)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("nodes:", nodes)

	rand.Seed(time.Now().UTC().UnixNano())
	randomNode := nodes[rand.Intn(len(nodes))]

	config, err := makeSSHConfig("ubuntu", *sshKey)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Created SSH client configuration: %#v", config)

	t := &tunnel{
		Config: config,
		Local:  "localhost:6443",
		Server: randomNode + ":22",
		Remote: "10.16.0.1:443",
	}

	log.Printf("Created SSH tunnel configuration: %#v", t)

	t.Listener, err = net.Listen("tcp", t.Local)
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("Started processing tunnel traffic")

	stopForwarder := make(chan struct{})
	go func() {
		log.Printf("Caught %s", <-signals)
		close(stopForwarder)
		if err := client.deleteProxy(); err != nil {
			log.Println(err)
		}
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
		if err := t.Listener.Close(); err != nil {
			log.Println(err)
		}
		log.Println("Tunnel closed")

	}()

	tunnelExit := make(chan struct{})
	go func() {
		if err := t.start(); err != nil {
			log.Fatalln(err)
		}
		close(tunnelExit)
	}()

	log.Printf("You can use `kubectl --kubeconfig=%q` to interact with the system", client.ConfigPath)

	go func() {
		if err := client.createProxy(stopForwarder); err != nil {
			log.Println(err)
		}
	}()

	<-tunnelExit
}
