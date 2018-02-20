package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/common/mtime"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
)

type mockServicesConfig struct {
	AcceptedOrgID string
	Flux          struct {
		Online    bool
		Connected bool
	}
	Scope struct {
		Online         bool
		NumberOfProbes int
	}
	Prom struct {
		Online          bool
		NumberOfMetrics int
	}
	Net struct {
		Online        bool
		NumberOfPeers int
	}
}

// MockServices handles all service endpoints in one server for testing purposes.
func MockServices(config *mockServicesConfig) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(user.OrgIDHeaderName)
		if orgID != config.AcceptedOrgID {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var resp interface{}
		switch r.RequestURI {
		case "/api/flux/v6/status":
			if config.Flux.Online {
				resp = map[string]interface{}{
					"fluxd": map[string]interface{}{
						"connected": config.Flux.Connected,
					},
				}
			}
		case "/api/probes":
			if config.Scope.Online {
				probes := []interface{}{}
				for i := 0; i < config.Scope.NumberOfProbes; i++ {
					probes = append(probes, struct{}{})
				}
				resp = probes
			}
		case "/api/probes?sparse":
			if config.Scope.Online {
				resp = config.Scope.NumberOfProbes > 0
			}
		case "/api/prom/user_stats":
			if config.Prom.Online {
				resp = map[string]interface{}{
					"ingestionRate": config.Prom.NumberOfMetrics / 4,
					"numSeries":     config.Prom.NumberOfMetrics,
				}
			}
		case "/api/net/peer":
			if config.Net.Online {
				peers := []interface{}{}
				for i := 0; i < config.Net.NumberOfPeers; i++ {
					peers = append(peers, struct{}{})
				}
				resp = peers
			}
		}
		if resp != nil {
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

func getOrgServiceStatus(t *testing.T, sparse bool, user *users.User, org *users.Organization) map[string]interface{} {
	w := httptest.NewRecorder()
	api := "/api/users/org/" + org.ExternalID + "/status"
	if sparse {
		api = api + "?sparse"
	}
	r := requestAs(t, user, "GET", api, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func assertCount(t *testing.T, keys int, sparse bool, count int, v interface{}, key string) {
	m, ok := v.(map[string]interface{})
	if !ok {
		assert.FailNow(t, "incorrect structure", "expected map, got %v", v)
	}
	if keys > 0 {
		assert.Equal(t, keys, len(m), "expected map with %d key(s), got %v", keys, m)
	}
	f, ok := m[key].(float64)
	if !ok {
		assert.FailNow(t, "incorrect structure", "expected float, got %v", m[key])
	}
	if sparse {
		assert.True(t, (count == 0 && f == 0) || (count > 0 && f > 0))
	} else {
		assert.Equal(t, float64(count), f)
	}
}

func assertGetOrgServiceStatus(t *testing.T, sparse bool, user *users.User, org *users.Organization, cfg *mockServicesConfig, now interface{}) {
	body := getOrgServiceStatus(t, sparse, user, org)
	assert.Equal(t, 6, len(body))
	assert.Equal(t, (cfg.Flux.Connected ||
		cfg.Scope.NumberOfProbes > 0 ||
		cfg.Prom.NumberOfMetrics > 0 ||
		cfg.Net.NumberOfPeers > 0), body["connected"])
	assert.Equal(t, now, body["firstSeenConnectedAt"])
	assert.Equal(t, map[string]interface{}{
		"firstSeenConnectedAt": now,
		"fluxsvc":              map[string]interface{}{},
		"fluxd": map[string]interface{}{
			"connected": cfg.Flux.Connected,
		},
		"git": map[string]interface{}{
			"configured": false,
			"config":     nil,
		},
	}, body["flux"])
	assertCount(t, 2, sparse, cfg.Scope.NumberOfProbes, body["scope"], "numberOfProbes")
	assertCount(t, -1, sparse, cfg.Prom.NumberOfMetrics, body["prom"], "numberOfMetrics")
	assertCount(t, 2, sparse, cfg.Net.NumberOfPeers, body["net"], "numberOfPeers")
}

func testGetOrgServiceStatus(t *testing.T, sparse bool) {
	now := time.Date(2017, 1, 1, 1, 1, 0, 0, time.UTC)
	cfg := &mockServicesConfig{}
	mockServices := MockServices(cfg)
	setupWithMockServices(t,
		mockServices.URL+"/api/flux/v6/status",
		mockServices.URL+"/api/probes",
		mockServices.URL+"/api/prom/user_stats",
		mockServices.URL+"/api/net/peer",
	)
	defer cleanup(t)

	user, org := getOrg(t)
	cfg.AcceptedOrgID = org.ID

	// Test when services are down.
	{
		body := getOrgServiceStatus(t, sparse, user, org)
		assert.Equal(t, false, body["connected"])
		assert.Nil(t, body["firstSeenConnectedAt"])
		for _, component := range []string{"flux", "scope", "prom", "net"} {
			v, ok := body[component]
			if !ok {
				assert.FailNow(t, "incorrect structure", "missing '%s' element", component)
			}
			vMap, ok := v.(map[string]interface{})
			if !ok {
				assert.FailNow(t, "incorrect structure", "'%s' element is not a map: %v", component, v)
			}
			assert.Equal(t, "Unexpected status code: 500", vMap["error"])
		}
	}

	// Test when services are online but not connected.
	{
		cfg.Flux.Online = true
		cfg.Scope.Online = true
		cfg.Prom.Online = true
		cfg.Net.Online = true

		assertGetOrgServiceStatus(t, sparse, user, org, cfg, nil)
	}

	// Test when services are online and connected.
	{
		cfg.Flux.Connected = true
		cfg.Scope.NumberOfProbes = 3
		cfg.Prom.NumberOfMetrics = 4
		cfg.Net.NumberOfPeers = 2

		mtime.NowForce(now)
		assertGetOrgServiceStatus(t, sparse, user, org, cfg, now.Format(time.RFC3339))
		mtime.NowReset()
	}

	// Test when services are then disconnected.
	{
		cfg.Flux.Connected = false
		cfg.Scope.NumberOfProbes = 0
		cfg.Prom.NumberOfMetrics = 0
		cfg.Net.NumberOfPeers = 0

		assertGetOrgServiceStatus(t, sparse, user, org, cfg, now.Format(time.RFC3339))
	}
}

func Test_GetOrgServiceStatus(t *testing.T) {
	testGetOrgServiceStatus(t, false)
}

func Test_GetOrgServiceStatusSparse(t *testing.T) {
	testGetOrgServiceStatus(t, true)
}
