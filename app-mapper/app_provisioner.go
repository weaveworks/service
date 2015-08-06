package main

import (
	"errors"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

type appProvisioner interface {
	fetchApp() error
	runApp(appID string) (string, error)
	destroyApp(appID string) error
	isAppRunning(appID string) (bool, error)
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

func (p *dockerProvisioner) fetchApp() error {
	options := docker.PullImageOptions{
		Repository: p.options.appConfig.Image,
		Tag:        "latest",
	}
	return p.client.PullImage(options, docker.AuthConfiguration{})
}

func (p *dockerProvisioner) runApp(appID string) (ret string, err error) {
	createOptions := docker.CreateContainerOptions{
		Name:       appID,
		Config:     &p.options.appConfig,
		HostConfig: &p.options.hostConfig,
	}
	c, err := p.client.CreateContainer(createOptions)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			p.destroyApp(c.ID)
		}
	}()

	err = p.client.StartContainer(c.ID, &p.options.hostConfig)
	if err != nil {
		return
	}

	// Wait until the app is running
	runDeadline := time.Now().Add(p.options.runTimeout)
	for !c.State.Running {
		c, err = p.client.InspectContainer(appID)
		if err != nil {
			return
		}
		if time.Now().After(runDeadline) {
			err = errDockerRunTimeout
			return
		}
		time.Sleep(time.Millisecond * 100)
	}

	ret = c.Config.Hostname + "." + c.Config.Domainname
	return
}

func (p *dockerProvisioner) isAppRunning(appID string) (bool, error) {
	c, err := p.client.InspectContainer(appID)
	if err != nil {
		return false, err
	}
	return c.State.Running, nil
}

func (p *dockerProvisioner) destroyApp(appID string) error {
	options := docker.RemoveContainerOptions{
		ID:            appID,
		RemoveVolumes: true,
		Force:         true,
	}
	return p.client.RemoveContainer(options)
}
