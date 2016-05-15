package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	_ "net/url"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"golang.org/x/crypto/ssh"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	unversionedapi "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	_ "k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/client/unversioned/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	_ "k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
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

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	stop := make(chan struct{}, 1)
	go func() {
		<-signals
		close(stop)
	}()

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

	log.Println("You can use `kubectl --kubeconfig=infra/dev/kubeconfig` to interact with the system")

	go func() {
		log.Println("Creating Kubernetes client...")

		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: "infra/dev/kubeconfig"}
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		config, err := kubeconfig.ClientConfig()
		if err != nil {
			log.Println(err)
			return
		}

		c, err := unversioned.New(config)
		if err != nil {
			log.Println(err)
			return
		}

		/*
			pods, err := c.Pods(api.NamespaceDefault).List(api.ListOptions{})
			if err != nil {
				log.Println(err)
				return
			}

			log.Printf("%#v", pods)
		*/

		ec, err := unversioned.NewExtensions(config)
		if err != nil {
			log.Println(err)
			return
		}

		/*
			deployments, err := ec.Deployments(api.NamespaceDefault).List(api.ListOptions{})
			if err != nil {
				log.Println(err)
				return
			}
			log.Printf("%#v", deployments)
		*/

		l := map[string]string{"name": "socksproxy-ilya"}

		log.Println("Creating socksproxy deployment...")
		zero := int64(0)
		d := &extensions.Deployment{
			ObjectMeta: api.ObjectMeta{
				Name: "socksproxy-ilya",
			},
			Spec: extensions.DeploymentSpec{
				Replicas: 1,
				Selector: &unversionedapi.LabelSelector{MatchLabels: l},
				Template: api.PodTemplateSpec{
					ObjectMeta: api.ObjectMeta{Labels: l},
					Spec: api.PodSpec{
						TerminationGracePeriodSeconds: &zero,
						Containers: []api.Container{
							{
								Name:  "socksproxy",
								Image: "weaveworks/socksproxy:latest",
							},
						},
					},
				},
			},
		}
		_, err = ec.Deployments(api.NamespaceDefault).Create(d)
		if err != nil {
			log.Println(err)
			if errors.IsAlreadyExists(err) {
				log.Println("Deployment of socksproxy with name \"socksproxy-ilya\" already exists - cleaning up...")
				err = ec.Deployments(api.NamespaceDefault).Delete("socksproxy-ilya", &api.DeleteOptions{})
				if err != nil {
					log.Println(err)
					return
				}
				log.Println("Creating socksproxy deployment...")
				_, err = ec.Deployments(api.NamespaceDefault).Create(d)
				if err != nil {
					log.Println(err)
					return
				}
			} else {
				return
			}
			go func() {
				<-signals
				log.Println("Deleting deployment of socksproxy...")
				err = ec.Deployments(api.NamespaceDefault).Delete("socksproxy-ilya", &api.DeleteOptions{})
				if err != nil {
					log.Println(err)
					return
				}
			}()
		}

		pods, err := c.Pods(api.NamespaceDefault).List(api.ListOptions{LabelSelector: labels.Set(l).AsSelector()})
		if err != nil {
			log.Println(err)
			return
		}

		podName := pods.Items[0].ObjectMeta.Name

		pod, err := c.Pods(api.NamespaceDefault).Get(podName)
		if err != nil {
			log.Println(err)
			return
		}

		for pod.Status.Phase != api.PodRunning {
			pod, err = c.Pods(api.NamespaceDefault).Get(podName)
			if err != nil {
				log.Println(err)
				return
			}
			time.Sleep(2 * time.Second)
		}

		req := c.RESTClient.Post().Resource("pods").Namespace(api.NamespaceDefault).Name(pod.Name).SubResource("portforward")

		ports := []string{"8000", "8080"}

		dialer, err := remotecommand.NewExecutor(config, "POST", req.URL())
		if err != nil {
			log.Println(err)
			return
		}

		fw, err := portforward.New(dialer, ports, stop, os.Stdout, os.Stderr)
		if err != nil {
			log.Println(err)
			return
		}

		err = fw.ForwardPorts()
		if err != nil {
			log.Println(err)
			return
		}
	}()

	<-tunnelExit
}
