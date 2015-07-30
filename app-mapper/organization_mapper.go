package main

type organizationMapper interface {
	getOrganizationsHost(orgID string) (string, error)
}

type constantMapper struct {
	targetHost string
}

func (m *constantMapper) getOrganizationsHost(orgID string) (string, error) {
	return m.targetHost, nil
}

type dbMapper struct {
	dbHost string
}

func (m *dbMapper) getOrganizationsHost(orgID string) (string, error) {
	// TODO
	return "", nil
}
