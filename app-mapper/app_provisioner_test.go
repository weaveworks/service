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

const appPort = "80"

func testAppProvisioner(t *testing.T, appID string, test func(p appProvisioner, host string)) {
	echoAppConfig := docker.Config{
		Image: "busybox",
		Cmd:   strings.Split("busybox nc -ll -p "+appPort+" -e cat", " "),
	}
	o := dockerProvisionerOptions{
		appConfig:     echoAppConfig,
		runTimeout:    defaultDockerRunTimeout,
		clientTimeout: defaultDockerClientTimeout,
	}
	p, err := newDockerProvisioner("unix:///var/run/weave.sock", o)
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
		conn, err := net.Dial("tcp", net.JoinHostPort(host, appPort))
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
