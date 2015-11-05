package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	scope "github.com/weaveworks/scope/xfer"
	k8sAPI "k8s.io/kubernetes/pkg/api"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
	k8sFields "k8s.io/kubernetes/pkg/fields"
	k8sLabels "k8s.io/kubernetes/pkg/labels"
)

type appProvisioner interface {
	fetchApp() error
	runApp(appID string) (host string, err error)
	destroyApp(appID string) error
	isAppReady(appID string) (ok bool, err error)
}

var errRunTimeout = errors.New("app provisioner: run timeout")

//
// Docker
//

type dockerProvisioner struct {
	client  *docker.Client
	options dockerProvisionerOptions
}

type dockerProvisionerOptions struct {
	appConfig     docker.Config
	hostConfig    docker.HostConfig
	clientTimeout time.Duration
}

func newDockerProvisioner(dockerHost string, options dockerProvisionerOptions) (*dockerProvisioner, error) {
	client, err := docker.NewClient(dockerHost)
	if err != nil {
		return nil, err
	}
	client.HTTPClient.Timeout = options.clientTimeout
	return &dockerProvisioner{client, options}, nil
}

func (p *dockerProvisioner) fetchApp() (err error) {
	_, err = p.client.InspectImage(p.options.appConfig.Image)
	if err == nil || err != docker.ErrNoSuchImage {
		return err
	}

	repository, tag := docker.ParseRepositoryTag(p.options.appConfig.Image)
	if tag == "" {
		tag = "latest"
	}
	pullImageOptions := docker.PullImageOptions{
		Repository: repository,
		Tag:        tag,
	}
	return p.client.PullImage(pullImageOptions, docker.AuthConfiguration{})
}

func (p *dockerProvisioner) runApp(appID string) (string, error) {
	createOptions := docker.CreateContainerOptions{
		Name:       getAppName(appID),
		Config:     &p.options.appConfig,
		HostConfig: &p.options.hostConfig,
	}
	spawnApp := func() error {
		container, err := p.client.CreateContainer(createOptions)
		if err != nil {
			return err
		}
		if err = p.client.StartContainer(container.ID, &p.options.hostConfig); err != nil {
			return err
		}
		return nil
	}
	if err := runApp(appID, p, spawnApp); err != nil {
		return "", err
	}
	return p.getAppHostname(appID)
}

func (p *dockerProvisioner) isAppReady(appID string) (bool, error) {
	hostname, err := p.getAppHostname(appID)
	if err != nil {
		return false, err
	}
	return pingScopeApp(hostname)
}

func (p *dockerProvisioner) destroyApp(appID string) error {
	options := docker.RemoveContainerOptions{
		ID:            getAppName(appID),
		RemoveVolumes: true,
		Force:         true,
	}
	if _, err := p.client.InspectContainer(getAppName(appID)); err != nil {
		return err
	}
	return p.client.RemoveContainer(options)
}

func (p *dockerProvisioner) getAppHostname(appID string) (string, error) {
	c, err := p.client.InspectContainer(getAppName(appID))
	if err != nil {
		return "", err
	}
	return c.Config.Hostname + "." + c.Config.Domainname, nil
}

//
// Kubernetes
//

type k8sProvisionerOptions struct {
	appContainer  k8sAPI.Container
	clientTimeout time.Duration
}

type k8sProvisioner struct {
	client    *k8sClient.Client
	namespace string
	options   k8sProvisionerOptions
}

func newK8sProvisioner(options k8sProvisionerOptions) (*k8sProvisioner, error) {
	client, err := k8sClient.NewInCluster()
	if err != nil {
		return nil, err
	}
	// TODO: Honor options.clientTimeout. It cannot be easily done until
	// https://github.com/kubernetes/kubernetes/issues/16793 is resolved.
	return &k8sProvisioner{client, "default", options}, nil
}

func (p *k8sProvisioner) fetchApp() (err error) {
	// There doesn't seem to be a simple way to programmatically tell k8s to
	// prefetch a Docker image in all its nodes. So, the image will be
	// lazilly fetched on runApp.
	//
	// TODO: We could force pulling the image in each node by spawning an
	// ephemeral pod which using the app-image and fixing the NodeName in its
	// PodSpec. Another option could be temporarily spawning a DaemonSet
	// using that image.
	return nil
}

func (p *k8sProvisioner) runApp(appID string) (hostname string, err error) {
	// Name used for all the labels and k8s object metadata. It will also
	// be the service hostname, which is derived from its name by the k8s
	// dns addon.
	name := getAppName(appID)

	labels := map[string]string{"name": name}
	selector := labels
	objMeta := k8sAPI.ObjectMeta{
		Name:   name,
		Labels: labels,
	}
	podTemplate := k8sAPI.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: k8sAPI.PodSpec{
			RestartPolicy: k8sAPI.RestartPolicyAlways,
			Containers:    []k8sAPI.Container{p.options.appContainer},
		},
	}
	rc := k8sAPI.ReplicationController{
		ObjectMeta: objMeta,
		Spec: k8sAPI.ReplicationControllerSpec{
			Replicas: 1,
			Selector: selector,
			Template: &podTemplate,
		},
	}
	service := k8sAPI.Service{
		ObjectMeta: objMeta,
		Spec: k8sAPI.ServiceSpec{
			Ports: []k8sAPI.ServicePort{
				k8sAPI.ServicePort{
					Protocol: "TCP",
					Port:     scope.AppPort,
				},
			},
			Selector: selector,
			// Doesn't matter since there's just one replica:
			SessionAffinity: k8sAPI.ServiceAffinityNone,
		},
	}

	spawnApp := func() error {
		if _, err := p.client.ReplicationControllers(p.namespace).Create(&rc); err != nil {
			return err
		}
		if _, err = p.client.Services(p.namespace).Create(&service); err != nil {
			return err
		}
		return nil
	}

	if err = runApp(appID, p, spawnApp); err != nil {
		return "", err
	}

	// We cannot provide the service FQDN because, AFAIK, the cluster domain is
	// not accessible to the k8s client
	return name, nil
}

func (p *k8sProvisioner) isAppReady(appID string) (bool, error) {
	hostname := getAppName(appID)
	return pingScopeApp(hostname)
}

func (p *k8sProvisioner) destroyApp(appID string) (err error) {
	var rcErr, podErr, serviceErr error
	name := getAppName(appID)

	// Delete replication controller
	var rc *k8sAPI.ReplicationController
	rcInterface := p.client.ReplicationControllers(p.namespace)
	if rc, rcErr = rcInterface.Get(name); rcErr == nil && rc != nil {
		logrus.Debugf("k8sProvisioner: destroyApp: destroying replication controller for app %q", appID)
		rcErr = rcInterface.Delete(name)
	}

	// Delete pod
	// Deleting a replication controller doesn't implicitly delete its
	// underlying pods (unlike how it happens during creation).
	labels := map[string]string{"name": name}
	labelSelector := k8sLabels.Set(labels).AsSelector()
	podInterface := p.client.Pods(p.namespace)
	var pods *k8sAPI.PodList
	if pods, podErr = podInterface.List(labelSelector, k8sFields.Everything()); podErr == nil && pods != nil {
		if podNum := len(pods.Items); podNum > 1 {
			logrus.Warnf("k8sProvisioner: destroyApp: unexpected number of pods (%d) for app %q", appID, podNum)
		}
		for _, pod := range pods.Items {
			logrus.Debugf("k8sProvisioner: destroyApp: destroying pod %q for app %q", pod.Name, appID)
			err := podInterface.Delete(pod.Name, &k8sAPI.DeleteOptions{})
			if err != nil {
				podErr = err
			}
		}
	}

	// Delete service
	var service *k8sAPI.Service
	serviceInterface := p.client.Services(p.namespace)
	if service, serviceErr = serviceInterface.Get(name); serviceErr == nil && service != nil {
		logrus.Debugf("k8sProvisioner: destroyApp: destroying service for app %q", appID)
		serviceErr = serviceInterface.Delete(name)
	}

	if rcErr != nil {
		return rcErr
	}
	if podErr != nil {
		return podErr
	}
	if serviceErr != nil {
		return serviceErr
	}

	return nil
}

//
// Helpers
//

func getAppName(appID string) string {
	return "scope-app-" + appID
}

func runApp(appID string, p appProvisioner, spawnApp func() error) error {
	err := spawnApp()
	if err != nil {
		logrus.Debugf("provisioner: rolling back app %q", appID)
		if err2 := p.destroyApp(appID); err2 != nil {
			logrus.Warnf("provisioner: error rolling back app %q: %v", appID, err2)
		}
	}
	return err
}

func pingScopeApp(hostname string) (bool, error) {
	pingTimeout := 200 * time.Millisecond
	hostPort := addPort(hostname, scope.AppPort)
	req, err := http.NewRequest("GET", "http://"+hostPort+"/api", nil)
	if err != nil {
		return false, err
	}
	client := &http.Client{
		Timeout: pingTimeout,
	}
	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK, nil
}
