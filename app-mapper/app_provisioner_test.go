// +build integration

package main

import (
	"net"
	"strings"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAppProvisioner(t *testing.T, appID string, test func(p appProvisioner, host string)) {
	echoAppConfig := docker.Config{
		Image:        "busybox",
		ExposedPorts: map[docker.Port]struct{}{docker.Port("8080"): {}},
		Cmd:          strings.Split("busybox nc -ll -p 8080 -e cat", " "),
		Hostname:     "localhost", // in production the hostname/domainname will be automatically provided by weave
	}
	hostConfig := docker.HostConfig{
		PortBindings: map[docker.Port][]docker.PortBinding{
			docker.Port("8080"): {docker.PortBinding{"127.0.0.1", "8080"}},
		},
	}
	o := dockerProvisionerOptions{
		appConfig:     echoAppConfig,
		hostConfig:    hostConfig,
		runTimeout:    defaultDockerRunTimeout,
		clientTimeout: defaultDockerClientTimeout,
	}
	p, err := newDockerProvisioner("unix:///var/run/docker.sock", o)
	require.NoError(t, err, "Cannot create provisioner")
	err = p.fetchApp()
	host, err := p.runApp(appID)
	require.NoError(t, err, "Cannot run app")

	test(p, host)

	err = p.destroyApp(appID)
	assert.NoError(t, err, "Cannot destroy app")
}

func TestSimpleProvisioning(t *testing.T) {
	const appID = "foo"

	test := func(p appProvisioner, host string) {
		running, err := p.isAppRunning(appID)
		assert.NoError(t, err, "Cannot check if app is running")
		assert.True(t, running, "App not running")
	}

	testAppProvisioner(t, appID, test)
}

func TestAccessRunningApp(t *testing.T) {
	const appID = "foo"
	const msg = "Tres tristes tigres"

	test := func(p appProvisioner, host string) {
		conn, err := net.Dial("tcp", net.JoinHostPort(host, "8080"))
		require.NoError(t, err, "Cannot contact app")
		defer conn.Close()
		_, err = conn.Write([]byte(msg))
		assert.NoError(t, err, "Cannot write to app")
		receivedMsg := make([]byte, len(msg))
		_, err = conn.Read(receivedMsg)
		assert.NoError(t, err, "Cannot read from app")
		assert.Equal(t, msg, string(receivedMsg))
	}

	testAppProvisioner(t, appID, test)
}
