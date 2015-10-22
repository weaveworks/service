// +build integration

package main

import (
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAppProvisioner(t *testing.T, appID string, test func(p appProvisioner, host string)) {
	appConfig := docker.Config{
		Image: defaultAppImage,
	}
	o := dockerProvisionerOptions{
		appConfig:     appConfig,
		runTimeout:    defaultDockerRunTimeout,
		clientTimeout: defaultDockerClientTimeout,
	}
	p, err := newDockerProvisioner("unix:///var/run/weave/weave.sock", o)
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
		require.NoError(t, err, "Cannot check if app is running")
		assert.True(t, running, "App not running")
	}

	testAppProvisioner(t, appID, test)
}

func TestIsAppReady(t *testing.T) {
	const appID = "foo"

	test := func(p appProvisioner, host string) {
		// Let the app boot
		time.Sleep(2 * time.Second)
		ready, err := p.isAppReady(appID)
		require.NoError(t, err, "Cannot check if app is ready")
		assert.True(t, ready, "App not running")
	}

	testAppProvisioner(t, appID, test)
}
