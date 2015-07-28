package main

type OrganizationMapper interface {
	GetOrganizationsHost(orgId string) (string, error)
}

type ConstantMapper struct {
	TargetHost string
}

func (m *ConstantMapper) GetOrganizationsHost(orgId string) (string, error) {
	return m.TargetHost, nil
}

type DBMapper struct {
	DBHost string
}

func (m *DBMapper) GetOrganizationsHost(orgId string) (string, error) {
	// TODO
	return "", nil
}
