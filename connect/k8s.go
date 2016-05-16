package main

import (
	"log"
	"os"
	"time"

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
