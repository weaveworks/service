package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	scope "github.com/weaveworks/scope/xfer"
)

type appProvisioner interface {
	fetchApp() error
	runApp(appID string) (host string, err error)
	destroyApp(appID string) error
	isAppRunning(appID string) (bool, error)
	isAppReady(appID string) (bool, error)
}

type dockerProvisioner struct {
	client  *docker.Client
	options dockerProvisionerOptions
}

type dockerProvisionerOptions struct {
	appConfig     docker.Config
	hostConfig    docker.HostConfig
	runTimeout    time.Duration
	clientTimeout time.Duration
}

var errDockerRunTimeout = errors.New("docker app provisioner: run timeout")

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

func (p *dockerProvisioner) runApp(appID string) (hostname string, err error) {
	createOptions := docker.CreateContainerOptions{
		Name:       appContainerName(appID),
		Config:     &p.options.appConfig,
		HostConfig: &p.options.hostConfig,
	}
	container, err := p.client.CreateContainer(createOptions)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			if err2 := p.destroyApp(appID); err2 != nil {
				logrus.Warnf("docker provisioner: destroy app %q: %v", appID, err2)
			}
		}
	}()

	if err = p.client.StartContainer(container.ID, &p.options.hostConfig); err != nil {
		return
	}

	// Wait until the app is running
	runDeadline := time.Now().Add(p.options.runTimeout)
	for !container.State.Running {
		container, err = p.client.InspectContainer(createOptions.Name)
		if err != nil {
			return
		}
		if time.Now().After(runDeadline) {
			err = errDockerRunTimeout
			return
		}
		time.Sleep(time.Millisecond * 100)
	}

	hostname = containerFQDN(container)
	return
}

func appContainerName(appID string) string {
	return "scope-app-" + appID
}

func (p *dockerProvisioner) isAppRunning(appID string) (ok bool, err error) {
	c, err := p.client.InspectContainer(appContainerName(appID))
	if err != nil {
		return false, err
	}
	return c.State.Running, nil
}

func (p *dockerProvisioner) isAppReady(appID string) (bool, error) {
	c, err := p.client.InspectContainer(appContainerName(appID))
	if err != nil {
		return false, err
	}
	return pingScopeApp(containerFQDN(c))
}

func (p *dockerProvisioner) destroyApp(appID string) (err error) {
	options := docker.RemoveContainerOptions{
		ID:            appContainerName(appID),
		RemoveVolumes: true,
		Force:         true,
	}
	return p.client.RemoveContainer(options)
}

func containerFQDN(c *docker.Container) string {
	return c.Config.Hostname + "." + c.Config.Domainname
}

func pingScopeApp(host string) (bool, error) {
	pingTimeout := 200 * time.Millisecond
	hostPort := addPort(host, scope.AppPort)
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
