// +build integration
// +build docker k8s

package main

import (
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sAPI "k8s.io/kubernetes/pkg/api"
)

func newTestProvisioner(t *testing.T) appProvisioner {
	generalOptions := appProvisionerOptions{
		runTimeout:    defaultProvisionerRunTimeout,
		clientTimeout: defaultProvisionerClientTimeout,
	}

	if isDockerIntegrationTest {
		appConfig := docker.Config{
			Image: defaultAppImage,
		}
		o := dockerProvisionerOptions{
			appConfig:             appConfig,
			appProvisionerOptions: generalOptions,
		}
		p, err := newDockerProvisioner("unix:///var/run/weave/weave.sock", o)
		require.NoError(t, err, "Cannot create docker provisioner")
		return p
	}

	if isK8sIntegrationTest {
		options := k8sProvisionerOptions{
			appContainer: k8sAPI.Container{
				Name:  "scope",
				Image: defaultAppImage,
				Args:  []string{"--no-probe"},
			},
			appProvisionerOptions: generalOptions,
		}

		p, err := newK8sProvisioner(options)
		require.NoError(t, err, "Cannot create k8s provisioner")
		return p
	}

	return nil
}

func runProvisionerTest(t *testing.T, appID string, p appProvisioner, test func(p appProvisioner, host string)) {
	err := p.fetchApp()
	require.NoError(t, err, "Cannot fetch app")
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

	p := newTestProvisioner(t)
	runProvisionerTest(t, appID, p, test)
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

	p := newTestProvisioner(t)
	runProvisionerTest(t, appID, p, test)
}
