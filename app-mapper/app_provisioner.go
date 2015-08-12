package main

import (
	"errors"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
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

func (p *dockerProvisioner) fetchApp() (err error) {
	defer func(begin time.Time) {
		val := strconv.FormatBool(err != nil)
		obs := float64(time.Since(begin).Nanoseconds())
		fetchAppLatency.WithLabelValues(val).Observe(obs)
	}(time.Now())

	_, err = p.client.InspectImage(p.options.appConfig.Image + ":latest")
	if err == nil || err != docker.ErrNoSuchImage {
		return err
	}

	pullImageOptions := docker.PullImageOptions{
		Repository: p.options.appConfig.Image,
		Tag:        "latest",
	}
	return p.client.PullImage(pullImageOptions, docker.AuthConfiguration{})
}

func (p *dockerProvisioner) runApp(appID string) (hostname string, err error) {
	defer func(begin time.Time) {
		val := strconv.FormatBool(err != nil)
		obs := float64(time.Since(begin).Nanoseconds())
		runAppLatency.WithLabelValues(val).Observe(obs)
	}(time.Now())

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

	hostname = container.Config.Hostname + "." + container.Config.Domainname
	return
}

func appContainerName(appID string) string {
	return "scope-app-" + appID
}

func (p *dockerProvisioner) isAppRunning(appID string) (ok bool, err error) {
	defer func(begin time.Time) {
		val := strconv.FormatBool(err != nil)
		obs := float64(time.Since(begin).Nanoseconds())
		isAppRunningLatency.WithLabelValues(val).Observe(obs)
	}(time.Now())

	c, err := p.client.InspectContainer(appContainerName(appID))
	if err != nil {
		return false, err
	}
	return c.State.Running, nil
}

func (p *dockerProvisioner) destroyApp(appID string) (err error) {
	defer func(begin time.Time) {
		val := strconv.FormatBool(err != nil)
		obs := float64(time.Since(begin).Nanoseconds())
		destroyAppLatency.WithLabelValues(val).Observe(obs)
	}(time.Now())

	options := docker.RemoveContainerOptions{
		ID:            appContainerName(appID),
		RemoveVolumes: true,
		Force:         true,
	}
	return p.client.RemoveContainer(options)
}
