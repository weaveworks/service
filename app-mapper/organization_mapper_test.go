// +build integration

package main

import (
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type orgHostNameRow struct {
	orgID    string
	hostname string
	isReady  bool
}

func testOrgMapper(t *testing.T, initialMapping []orgHostNameRow, p appProvisioner, test func(organizationMapper)) {
	db, err := sqlx.Open("postgres", defaultDBURI)
	require.NoError(t, err, "Cannot initialize database client")
	dbMapper := newDBMapper(db, p)
	_, err = dbMapper.db.Exec("DELETE FROM org_hostname")
	require.NoError(t, err, "Cannot wipe DB")

	for _, orgHost := range initialMapping {
		_, err = dbMapper.db.Exec(
			"INSERT INTO org_hostname VALUES ($1, $2, $3);",
			orgHost.orgID,
			orgHost.hostname,
			orgHost.isReady,
		)
		require.NoError(t, err, "Cannot initialize orgHost mapping")
	}

	test(dbMapper)
}

func TestSimpleMapping(t *testing.T) {
	const (
		orgID      = "foo"
		targetHost = "target.com"
	)

	m := []orgHostNameRow{{orgID, targetHost, true}}

	p := mockProvisioner{
		mockRunApp: func(string) (string, error) {
			assert.Fail(t, "Provisioner shouldn't be invoked")
			return "", nil
		},
	}

	test := func(m organizationMapper) {
		hostInfo, err := m.getOrganizationsHost(orgID)
		require.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, hostInfo.HostName, "Unexpected host")
		assert.Equal(t, true, hostInfo.IsReady, "Unexpected isReady")
	}

	testOrgMapper(t, m, p, test)
}

func TestProvisioning(t *testing.T) {
	const (
		orgID      = "foo"
		targetHost = "target.com"
	)

	var appReady bool

	m := []orgHostNameRow{}

	provisionerCalled := false
	p := mockProvisioner{
		mockRunApp: func(string) (string, error) {
			provisionerCalled = true
			return targetHost, nil
		},
		mockIsAppReady: func(string) (bool, error) {
			return appReady, nil
		},
	}

	test := func(m organizationMapper) {
		appReady = false

		hostInfo, err := m.getOrganizationsHost(orgID)
		require.NoError(t, err, "Unsuccessful query")
		assert.True(t, provisionerCalled, "Provisioner must be called since the hostname didn't exist")
		assert.Equal(t, targetHost, hostInfo.HostName, "Unexpected Hostname")
		assert.False(t, hostInfo.IsReady, "App can't be ready when we map it for the first time")

		// Check that the mapping is maintained across requests
		// and that the provisioner is not called once the
		// provisioning happens the first time
		provisionerCalled = false
		hostInfo, err = m.getOrganizationsHost(orgID)
		require.NoError(t, err, "Unsuccessful query")
		assert.False(t, provisionerCalled, "Provisioner shouldn't be called since the hostname exists")
		assert.Equal(t, targetHost, hostInfo.HostName, "Unexpected Hostname")
		assert.False(t, hostInfo.IsReady, "App not set to be ready")

		// Make the  app ready
		appReady = true
		hostInfo, err = m.getOrganizationsHost(orgID)
		require.NoError(t, err, "Unsuccessful query")
		assert.False(t, provisionerCalled, "Provisioner shouldn't be called since the hostname exists")
		assert.Equal(t, targetHost, hostInfo.HostName, "Unexpected Hostname")
		assert.True(t, hostInfo.IsReady, "App should finally be ready")
	}

	testOrgMapper(t, m, p, test)
}

func TestNoProvisioningSideEffects(t *testing.T) {
	const (
		existingOrgID      = "foo"
		existingTargetHost = "target.com"
		newOrgID           = "bar"
		newTargetHost      = "target2.com"
	)

	m := []orgHostNameRow{{existingOrgID, existingTargetHost, true}}

	p := mockProvisioner{
		mockRunApp: func(string) (string, error) {
			return newTargetHost, nil
		},
		mockIsAppReady: func(string) (bool, error) {
			return true, nil
		},
	}

	test := func(m organizationMapper) {
		hostInfo, err := m.getOrganizationsHost(newOrgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, newTargetHost, hostInfo.HostName)

		// Check that existing mappings are maintained across requests
		hostInfo, err = m.getOrganizationsHost(existingOrgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, existingTargetHost, hostInfo.HostName)
	}

	testOrgMapper(t, m, p, test)
}

func TestFailingProvisioner(t *testing.T) {
	const orgID = "foo"

	m := []orgHostNameRow{}

	p := mockProvisioner{
		mockRunApp: func(string) (string, error) {
			return "", errors.New("whatever")
		},
	}

	test := func(m organizationMapper) {
		_, err := m.getOrganizationsHost(orgID)
		assert.Error(t, err, "Unexpected successful mapping")
	}

	testOrgMapper(t, m, p, test)
}
