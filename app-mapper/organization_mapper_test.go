// +build integration

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type initialState struct {
	hosts   []string
	mapping []dbOrgHost
}

func testOrgMapper(t *testing.T, s *initialState, test func(organizationMapper)) {
	const dbURI = "postgres://postgres@localhost/app_mapper?sslmode=disable"
	dbMapper, err := newDBMapper(dbURI)
	assert.NoError(t, err, "Cannot connect to DB")

	_, err = dbMapper.db.Exec("DELETE FROM org_host")
	assert.NoError(t, err, "Cannot wipe DB")

	_, err = dbMapper.db.Exec("DELETE FROM hosts")
	assert.NoError(t, err, "Cannot wipe DB")

	for _, host := range s.hosts {
		_, err = dbMapper.db.Exec("INSERT INTO hosts VALUES ($1);", host)
		assert.NoError(t, err, "Cannot initialize hosts")

	}

	for _, orgHost := range s.mapping {
		_, err = dbMapper.db.NamedExec("INSERT INTO org_host VALUES (:organization_id, :host);", orgHost)
		assert.NoError(t, err, "Cannot initialize orgHost mapping")
	}

	test(dbMapper)
}

func TestSimpleMapping(t *testing.T) {
	const (
		orgID      = "foo"
		targetHost = "target.com"
	)

	s := initialState{
		hosts:   []string{targetHost},
		mapping: []dbOrgHost{{orgID, targetHost}},
	}

	test := func(m organizationMapper) {
		host, err := m.getOrganizationsHost(orgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, host)
	}

	testOrgMapper(t, &s, test)
}

func TestAllocation(t *testing.T) {
	const (
		orgID      = "foo"
		targetHost = "target.com"
	)

	s := initialState{
		hosts:   []string{targetHost},
		mapping: []dbOrgHost{},
	}

	test := func(m organizationMapper) {
		host, err := m.getOrganizationsHost(orgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, host)

		// Check that the mapping is maintained across requests
		host, err = m.getOrganizationsHost(orgID)
		assert.NoError(t, err, "Unsuccessful query")
		assert.Equal(t, targetHost, host)
	}

	testOrgMapper(t, &s, test)
}

func TestHostLimit(t *testing.T) {
	const (
		existingOrgID      = "foo"
		existingTargetHost = "target.com"
		newOrgID           = "bar"
	)

	s := initialState{
		hosts:   []string{existingTargetHost},
		mapping: []dbOrgHost{{existingOrgID, existingTargetHost}},
	}

	test := func(m organizationMapper) {
		_, err := m.getOrganizationsHost(newOrgID)
		assert.Error(t, err, "There shouldn't be hosts left")
		host, err := m.getOrganizationsHost(existingOrgID)
		assert.NoError(t, err, "Existing mapping shouldn't change")
		assert.Equal(t, existingTargetHost, host)
	}

	testOrgMapper(t, &s, test)
}
