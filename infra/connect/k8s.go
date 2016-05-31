package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
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

type kubeClient struct {
	podName           string
	deploymentName    string
	deploymentCreated bool
	c                 *unversioned.Client
	ec                *unversioned.ExtensionsClient
	ConfigPath        string
	UserName          string
}

const (
	sleepPeriodSeconds  = 2
	timeoutAfterPeriods = 60
)

func (kube *kubeClient) createProxy(stop <-chan struct{}) error {
	log.Println("Creating Kubernetes client...")

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kube.ConfigPath}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeconfig.ClientConfig()
	if err != nil {
		return err
	}

	kube.c, err = unversioned.New(config)
	if err != nil {
		return err
	}

	kube.ec, err = unversioned.NewExtensions(config)
	if err != nil {
		return err
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
							Name:    "socksproxy",
							Image:   "weaveworks/socksproxy:latest",
							Command: []string{"/proxy", "-h", "*.*.svc.cluster.local", "-a", "scope.weave.works:frontend.default.svc.cluster.local"},
						},
					},
				},
			},
		},
	}
	kube.deploymentCreated = false
	_, err = kube.ec.Deployments(api.NamespaceDefault).Create(d)
	if errors.IsAlreadyExists(err) {
		log.Printf("Deployment of socksproxy with name %q already exists - cleaning up...", kube.deploymentName)
		err = kube.ec.Deployments(api.NamespaceDefault).Delete(kube.deploymentName, &api.DeleteOptions{})
		if err != nil {
			return err
		}
		log.Println("Creating socksproxy deployment...")
		_, err = kube.ec.Deployments(api.NamespaceDefault).Create(d)
		if err != nil {
			return err
		}
		kube.deploymentCreated = true
	} else if err != nil {
		return err
	}

	pods, err := kube.c.Pods(api.NamespaceDefault).List(api.ListOptions{LabelSelector: labels.Set(l).AsSelector()})
	if err != nil {
		return err
	}

	kube.podName = pods.Items[0].ObjectMeta.Name

	for timeout := 0; ; timeout++ {
		pod, err := kube.c.Pods(api.NamespaceDefault).Get(kube.podName)
		if err != nil {
			return err
		}
		if pod.Status.Phase == api.PodRunning {
			break
		}
		log.Printf("Waiting for pod %q... [pod.Status.Phase: %q]", kube.podName, pod.Status.Phase)
		time.Sleep(sleepPeriodSeconds * time.Second)
		if timeout >= timeoutAfterPeriods {
			return fmt.Errorf("Timeout after %d seconds!", sleepPeriodSeconds*timeoutAfterPeriods)
		}
	}

	log.Printf("Creating port forwarder for pod %q...", kube.podName)
	req := kube.c.RESTClient.Post().Resource("pods").Namespace(api.NamespaceDefault).Name(kube.podName).SubResource("portforward")

	ports := []string{"8000", "8080"}

	dialer, err := remotecommand.NewExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	fw, err := portforward.New(dialer, ports, stop, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	log.Printf("Configure your browser to use \"http://localhost:%s/proxy.pac\"", ports[1])

	if err := fw.ForwardPorts(); err != nil {
		return err
	}
	log.Println("Port forwarder terminated")
	return nil
}

func (kube *kubeClient) deleteProxy() error {
	if kube.deploymentCreated {
		log.Println("Deleting deployment of socksproxy...")
		if err := kube.ec.Deployments(api.NamespaceDefault).Delete(kube.deploymentName, &api.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}
