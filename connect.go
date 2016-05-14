package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"golang.org/x/crypto/ssh"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"

	_ "k8s.io/kubernetes/pkg/fields"
	_ "k8s.io/kubernetes/pkg/labels"
	_ "k8s.io/kubernetes/pkg/ssh"
)

type Endpoint struct {
	Host string
	Port int
}

func (endpoint *Endpoint) String() string {
	return fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
}

type SSHtunnel struct {
	Local  *Endpoint
	Server *Endpoint
	Remote *Endpoint

	Config *ssh.ClientConfig
}

func (tunnel *SSHtunnel) Start(ready chan bool) error {
	log.Println("Started processing tunnel traffic")
	listener, err := net.Listen("tcp", tunnel.Local.String())
	if err != nil {
		return err
	}
	defer listener.Close()
	ready <- true

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go tunnel.forward(conn)
	}
}

func (tunnel *SSHtunnel) forward(localConn net.Conn) {
	log.Printf("Forwarding connection: %v\n", localConn)
	serverConn, err := ssh.Dial("tcp", tunnel.Server.String(), tunnel.Config)
	if err != nil {
		log.Printf("Server dial error: %s\n", err)
		return
	}

	remoteConn, err := serverConn.Dial("tcp", tunnel.Remote.String())
	if err != nil {
		log.Printf("Remote dial error: %s\n", err)
		return
	}

	copyConn := func(writer, reader net.Conn) {
		s, err := io.Copy(writer, reader)
		if err != nil {
			log.Println("io.Copy error: %s", err)
		}
		log.Printf("Copied %d bytes for connection %v", s, reader)
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}

func parsePrivateKey(keyPath string) (ssh.Signer, error) {
	buff, _ := ioutil.ReadFile(keyPath)
	return ssh.ParsePrivateKey(buff)
}

func makeSshConfig(user, privateKeyPath string) (*ssh.ClientConfig, error) {
	key, err := parsePrivateKey(privateKeyPath)
	if err != nil {
		return nil, err
	}

	config := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	return &config, nil
}

func getNodes() (nodes []string, err error) {
	svc := ec2.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:KubernetesCluster"),
				Values: []*string{
					aws.String("kubernetes-devz"),
				},
			},
			{
				Name: aws.String("tag:Name"),
				Values: []*string{
					aws.String("kubernetes-node"),
				},
			},
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	}
	resp, err := svc.DescribeInstances(params)
	if err != nil {
		return
	}
	log.Println("Number of reservation sets:", len(resp.Reservations))
	for idx, res := range resp.Reservations {
		log.Println("Number of instances:", len(res.Instances))
		for _, inst := range resp.Reservations[idx].Instances {
			log.Println("Instance ID:", *inst.InstanceId)
			nodes = append(nodes, *inst.PublicIpAddress)
		}
	}
	return
}
func main() {

	nodes, err := getNodes()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("nodes:", nodes)
	rand.Seed(time.Now().UTC().UnixNano())

	localEndpoint := &Endpoint{
		Host: "localhost",
		Port: 6443,
	}

	serverEndpoint := &Endpoint{
		Host: nodes[rand.Intn(len(nodes))],
		Port: 22,
	}

	remoteEndpoint := &Endpoint{
		Host: "10.16.0.1",
		Port: 443,
	}

	config, err := makeSshConfig("ubuntu", "infra/dev/ec2_weaveworks.kubernetes-anywhere.us-east-1.pem")
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Created SSH client configuration: %#v\n", config)

	tunnel := &SSHtunnel{
		Config: config,
		Local:  localEndpoint,
		Server: serverEndpoint,
		Remote: remoteEndpoint,
	}

	log.Printf("Created SSH tunnel configuration: %#v\n", tunnel)

	tunnelReady, tunnelExit := make(chan bool), make(chan bool)

	go func() {
		err := tunnel.Start(tunnelReady)
		if err != nil {
			log.Fatalln(err)
		}
		tunnelExit <- true
	}()

	<-tunnelReady

	go func() {
		log.Println("Creating Kubernetes client...")

		//loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: "infra/dev/kubeconfig"}
		// if you want to change the loading rules (which files in which order), you can do so here

		configOverrides := &clientcmd.ConfigOverrides{}
		// if you want to change override values or bind them to flags, there are methods to help you

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err := kubeConfig.ClientConfig()
		if err != nil {
			log.Println(err)
			return
		}
		client, err := unversioned.New(config)
		pods, err := client.Pods(api.NamespaceDefault).List(api.ListOptions{})
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("%#v", pods)
	}()

	<-tunnelExit
}
