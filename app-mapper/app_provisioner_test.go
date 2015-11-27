// +build integration
// +build docker k8s

package main

import (
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kapi "k8s.io/kubernetes/pkg/api"
)

func newTestProvisioner(t *testing.T) appProvisioner {
	args := []string{"--no-probe"}

	if isDockerIntegrationTest {
		appConfig := docker.Config{
			Image: defaultAppImage,
			Cmd:   args,
		}
		o := dockerProvisionerOptions{
			appConfig:     appConfig,
			clientTimeout: defaultProvisionerClientTimeout,
		}
		p, err := newDockerProvisioner("unix:///var/run/weave/weave.sock", o)
		require.NoError(t, err, "Cannot create docker provisioner")
		return p
	}

	if isK8sIntegrationTest {
		options := k8sProvisionerOptions{
			appContainer: kapi.Container{
				Name:  "scope",
				Image: defaultAppImage,
				Args:  args,
			},
			clientTimeout: defaultProvisionerClientTimeout,
		}

		p, err := newK8sProvisioner(options)
		require.NoError(t, err, "Cannot create k8s provisioner")
		return p
	}

	t.Fatal("Unspecified app provisioner type")
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

	test := func(p appProvisioner, host string) {}

	p := newTestProvisioner(t)
	runProvisionerTest(t, appID, p, test)
}

func TestIsAppReady(t *testing.T) {
	const appID = "foo"

	test := func(p appProvisioner, host string) {
		// Wait indefinitely until the app is ready. If this hangs, the
		// testsuite timeout will simply kick in.
		for true {
			ready, err := p.isAppReady(appID)
			if err == nil && ready {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

	}

	p := newTestProvisioner(t)
	runProvisionerTest(t, appID, p, test)
}
