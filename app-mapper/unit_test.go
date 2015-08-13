package main

import (
	"net/http"
	"time"
)

var testProbeStorage probeStorage = &probeMemStorage{}

type authenticatorFunc func(r *http.Request, orgName string) (authenticatorResponse, error)

func (f authenticatorFunc) authenticateOrg(r *http.Request, orgName string) (authenticatorResponse, error) {
	return f(r, orgName)
}

func (f authenticatorFunc) authenticateProbe(r *http.Request) (authenticatorResponse, error) {
	return f(r, "")
}

type mockProvisioner func(appID string) (string, error)

func (f mockProvisioner) fetchApp() error {
	return nil
}

func (f mockProvisioner) runApp(appID string) (string, error) {
	return f(appID)
}

func (f mockProvisioner) destroyApp(string) error {
	return nil
}

func (f mockProvisioner) isAppRunning(string) (bool, error) {
	return true, nil
}

type probeMemStorage struct {
	probes []probe
}

func (s *probeMemStorage) getProbesFromOrg(orgID string) ([]probe, error) {
	var result []probe
	for _, probe := range s.probes {
		if probe.OrgID == orgID {
			result = append(result, probe)
		}
	}
	return result, nil
}

func (s *probeMemStorage) bumpProbeLastSeen(probeID string, orgID string) error {
	for _, probe := range s.probes {
		if probe.ID == probeID {
			probe.LastSeen = time.Now()
			return nil
		}
	}
	newProbe := probe{
		ID:       probeID,
		OrgID:    orgID,
		LastSeen: time.Now(),
	}

	s.probes = append(s.probes, newProbe)
	return nil
}
