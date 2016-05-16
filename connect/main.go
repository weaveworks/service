package main

import (
	_ "flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"
)

func main() {

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	client := &KubernetesClient{ConfigPath: "infra/dev/kubeconfig", UserName: os.Getenv("USER")}

	tunnelReady, tunnelExit, stopForwarder := make(chan bool, 1), make(chan bool, 1), make(chan struct{}, 1)

	nodes, err := getEc2Nodes("kubernetes-devz")
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("nodes:", nodes)

	rand.Seed(time.Now().UTC().UnixNano())
	randomNode := nodes[rand.Intn(len(nodes))]

	config, err := makeSshConfig("ubuntu", "infra/dev/ec2_weaveworks.kubernetes-anywhere.us-east-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Created SSH client configuration: %#v\n", config)

	tunnel := &SSHtunnel{
		Config: config,
		Local:  &Endpoint{Host: "localhost", Port: 6443},
		Server: &Endpoint{Host: randomNode, Port: 22},
		Remote: &Endpoint{Host: "10.16.0.1", Port: 443},
	}

	log.Printf("Created SSH tunnel configuration: %#v\n", tunnel)

	go func() {
		<-signals
		log.Println("Caught SIGINT...")
		close(stopForwarder)
		err := client.DeleteProxy()
		if err != nil {
			log.Println(err)
		}
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
		err = tunnel.Stop()
		if err != nil {
			log.Println(err)
		}

	}()

	go func() {
		err := tunnel.Start(tunnelReady)
		if err != nil {
			log.Fatalln(err)
		}
		tunnelExit <- true
	}()

	<-tunnelReady

	log.Printf("You can use `kubectl --kubeconfig=%q` to interact with the system", client.ConfigPath)

	go func() {
		err := client.CreateProxy(stopForwarder)
		if err != nil {
			log.Println(err)
		}
	}()

	<-tunnelExit
}
