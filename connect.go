package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
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
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/client/unversioned/portforward"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
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
	Local    *Endpoint
	Server   *Endpoint
	Remote   *Endpoint
	Config   *ssh.ClientConfig
	listener net.Listener
}

func (tunnel *SSHtunnel) Start(ready chan bool) (err error) {
	log.Println("Started processing tunnel traffic")
	tunnel.listener, err = net.Listen("tcp", tunnel.Local.String())
	if err != nil {
		return
	}

	ready <- true

	for {
		conn, err := tunnel.listener.Accept()
		if err != nil {
			return err
		}
		go tunnel.forward(conn)
	}
}

func (tunnel *SSHtunnel) Stop() (err error) {
	err = tunnel.listener.Close()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Tunnel closed")
	return
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
			log.Printf("I/O error: %s", err)
		}
		log.Printf("Copied %d bytes for connection %v", s, reader)
	}

	go copyConn(localConn, remoteConn)
	go copyConn(remoteConn, localConn)
}

func makeSshConfig(user, privateKeyPath string) (*ssh.ClientConfig, error) {
	buff, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(buff)
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

type KubernetesClient struct {
	podName        string
	deploymentName string
	c              *unversioned.Client
	ec             *unversioned.ExtensionsClient
	ConfigPath     string
	UserName       string
}

func (kube *KubernetesClient) CreateProxy(stop <-chan struct{}) (err error) {
	log.Println("Creating Kubernetes client...")

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kube.ConfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeconfig.ClientConfig()
	if err != nil {
		return
	}

	kube.c, err = unversioned.New(config)
	if err != nil {
		return
	}

	kube.ec, err = unversioned.NewExtensions(config)
	if err != nil {
		return
	}

	kube.deploymentName = "socksproxy-" + kube.UserName
	l := map[string]string{"name": kube.deploymentName}

	log.Println("Creating socksproxy deployment...")
	zero := int64(0)
	d := &extensions.Deployment{
		ObjectMeta: api.ObjectMeta{Name: kube.deploymentName},
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
	_, err = kube.ec.Deployments(api.NamespaceDefault).Create(d)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			log.Printf("Deployment of socksproxy with name %q already exists - cleaning up...", kube.deploymentName)
			err = kube.ec.Deployments(api.NamespaceDefault).Delete(kube.deploymentName, &api.DeleteOptions{})
			if err != nil {
				return
			}
			log.Println("Creating socksproxy deployment...")
			_, err = kube.ec.Deployments(api.NamespaceDefault).Create(d)
			if err != nil {
				return
			}
		} else {
			return
		}
	}

	pods, err := kube.c.Pods(api.NamespaceDefault).List(api.ListOptions{LabelSelector: labels.Set(l).AsSelector()})
	if err != nil {
		return
	}

	kube.podName = pods.Items[0].ObjectMeta.Name

	pod, err := kube.c.Pods(api.NamespaceDefault).Get(kube.podName)
	if err != nil {
		return
	}

	for pod.Status.Phase != api.PodRunning {
		log.Println("Waiting for pod %q...", kube.podName)
		time.Sleep(2 * time.Second)
		pod, err = kube.c.Pods(api.NamespaceDefault).Get(kube.podName)
		if err != nil {
			return
		}
	}

	log.Printf("Creating port forwarder for pod %q...", kube.podName)
	req := kube.c.RESTClient.Post().Resource("pods").Namespace(api.NamespaceDefault).Name(kube.podName).SubResource("portforward")

	ports := []string{"8000", "8080"}

	dialer, err := remotecommand.NewExecutor(config, "POST", req.URL())
	if err != nil {
		return
	}

	fw, err := portforward.New(dialer, ports, stop, os.Stdout, os.Stderr)
	if err != nil {
		return
	}

	err = fw.ForwardPorts()
	if err != nil {
		return
	}
	log.Println("Port forwarder terminated")
	return
}

func (kube *KubernetesClient) DeleteProxy() (err error) {
	log.Println("Deleting deployment of socksproxy...")
	err = kube.ec.Deployments(api.NamespaceDefault).Delete(kube.deploymentName, &api.DeleteOptions{})
	if err != nil {
		return
	}
	return
}

func getEc2Nodes(cluster string) (nodes []string, err error) {
	svc := ec2.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:KubernetesCluster"),
				Values: []*string{
					aws.String(cluster),
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

	client := &KubernetesClient{ConfigPath: "infra/dev/kubeconfig", UserName: "ilya"}

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
