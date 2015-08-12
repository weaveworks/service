// +build integration

package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testOrgMapper(t *testing.T, initialMapping []dbOrgHost, p appProvisioner, test func(organizationMapper)) {
	dbMapper := newDBMapper(testDB, p)

	_, err := dbMapper.db.Exec("DELETE FROM org_hostname")
	require.NoError(t, err, "Cannot wipe DB")

	for _, orgHost := range initialMapping {
		_, err = dbMapper.db.NamedExec("INSERT INTO org_hostname VALUES (:organization_id, :hostname);", orgHost)
		require.NoError(t, err, "Cannot initialize orgHost mapping")
	}

	test(dbMapper)
}

func TestSimpleMapping(t *testing.T) {
	const (
		orgID      = "foo"
		targetHost = "target.com"
	)

	m := []dbOrgHost{{orgID, targetHost}}

	p := mockProvisioner(func(string) (string, error) {
		assert.Fail(t, "Provisioner shouldn't be invoked")
		return "", nil
	})

	test := func(m organizationMapper) {
		host, err := m.getOrganizationsHost(orgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, host)
	}

	testOrgMapper(t, m, p, test)
}

func TestProvisioning(t *testing.T) {
	const (
		orgID      = "foo"
		targetHost = "target.com"
	)

	m := []dbOrgHost{}

	provisionerCalled := false
	p := mockProvisioner(func(string) (string, error) {
		provisionerCalled = true
		return targetHost, nil
	})

	test := func(m organizationMapper) {
		host, err := m.getOrganizationsHost(orgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, host)
		assert.True(t, provisionerCalled, targetHost)

		// Check that the mapping is maintained across requests
		// and that the provisioner is not called once the
		// provisioning happens the first time
		provisionerCalled = false
		host, err = m.getOrganizationsHost(orgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, host)
		assert.False(t, provisionerCalled, targetHost)
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

	m := []dbOrgHost{{existingOrgID, existingTargetHost}}

	p := mockProvisioner(func(string) (string, error) {
		return newTargetHost, nil
	})

	test := func(m organizationMapper) {
		host, err := m.getOrganizationsHost(newOrgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, newTargetHost, host)

		// Check that existing mappings are maintained across requests
		host, err = m.getOrganizationsHost(existingOrgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, existingTargetHost, host)
	}

	testOrgMapper(t, m, p, test)
}

func TestFailingProvisioner(t *testing.T) {
	const orgID = "foo"

	m := []dbOrgHost{}

	p := mockProvisioner(func(string) (string, error) {
		return "", errors.New("whatever")
	})

	test := func(m organizationMapper) {
		_, err := m.getOrganizationsHost(orgID)
		assert.Error(t, err, "Unexpected successful mapping")
	}

	testOrgMapper(t, m, p, test)
}
