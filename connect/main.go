package main

import (
	_ "flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"
)

/*
type flags struct {
	clusterName string
	kubeConfig  string
	sshKey      string
}
*/

func main() {

	//f := flags{}
	//flag.StringVar(&f.cluserName, "cluster-name", "

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	client := &kubeClient{ConfigPath: "infra/dev/kubeconfig", UserName: os.Getenv("USER")}

	tunnelReady, tunnelExit, stopForwarder := make(chan bool, 1), make(chan bool, 1), make(chan struct{}, 1)

	nodes, err := getEc2Nodes("kubernetes-devz")
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("nodes:", nodes)

	rand.Seed(time.Now().UTC().UnixNano())
	randomNode := nodes[rand.Intn(len(nodes))]

	config, err := makeSSHConfig("ubuntu", "infra/dev/ec2_weaveworks.kubernetes-anywhere.us-east-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Created SSH client configuration: %#v\n", config)

	t := &tunnel{
		Config: config,
		Local:  &endpoint{Host: "localhost", Port: 6443},
		Server: &endpoint{Host: randomNode, Port: 22},
		Remote: &endpoint{Host: "10.16.0.1", Port: 443},
	}

	log.Printf("Created SSH tunnel configuration: %#v\n", t)

	go func() {
		<-signals
		log.Println("Caught SIGINT...")
		close(stopForwarder)
		err := client.deleteProxy()
		if err != nil {
			log.Println(err)
		}
		log.Println("Waiting...")
		time.Sleep(5 * time.Second)
		err = t.stop()
		if err != nil {
			log.Println(err)
		}

	}()

	go func() {
		err := t.start(tunnelReady)
		if err != nil {
			log.Fatalln(err)
		}
		tunnelExit <- true
	}()

	<-tunnelReady

	log.Printf("You can use `kubectl --kubeconfig=%q` to interact with the system", client.ConfigPath)

	go func() {
		err := client.createProxy(stopForwarder)
		if err != nil {
			log.Println(err)
		}
	}()

	<-tunnelExit
}
