package main

import (
	"net/http"
	"time"
)

var isIntegrationTest = false

type authenticatorFunc func(r *http.Request, orgName string) (authenticatorResponse, error)

func (f authenticatorFunc) authenticateOrg(r *http.Request, orgName string) (authenticatorResponse, error) {
	return f(r, orgName)
}

func (f authenticatorFunc) authenticateProbe(r *http.Request) (authenticatorResponse, error) {
	return f(r, "")
}

type mockProvisioner struct {
	mockRunApp     func(appID string) (string, error)
	mockIsAppReady func(appID string) (bool, error)
}

func (mockProvisioner) fetchApp() error {
	return nil
}

func (m mockProvisioner) runApp(appID string) (string, error) {
	return m.mockRunApp(appID)
}

func (mockProvisioner) destroyApp(string) error {
	return nil
}

func (mockProvisioner) isAppRunning(string) (bool, error) {
	return true, nil
}

func (m mockProvisioner) isAppReady(appID string) (bool, error) {
	return m.mockIsAppReady(appID)
}

type memProbe struct {
	probe
	orgID string
}

type probeMemStorage struct {
	memProbes []memProbe
}

func (s *probeMemStorage) getProbesFromOrg(orgID string) ([]probe, error) {
	var result []probe
	for _, memProbe := range s.memProbes {
		if memProbe.orgID == orgID {
			result = append(result, probe{memProbe.ID, memProbe.LastSeen})
		}
	}
	return result, nil
}

func (s *probeMemStorage) bumpProbeLastSeen(probeID string, orgID string) error {
	for i, memProbe := range s.memProbes {
		if memProbe.ID == probeID {
			s.memProbes[i].LastSeen = time.Now()
			return nil
		}
	}
	newProbe := memProbe{
		probe: probe{probeID, time.Now()},
		orgID: orgID,
	}

	s.memProbes = append(s.memProbes, newProbe)
	return nil
}
