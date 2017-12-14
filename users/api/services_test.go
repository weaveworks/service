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
		case "/api/prom/api/v1/label/__name__/values":
			if config.Prom.Online {
				metrics := []interface{}{}
				for i := 0; i < config.Prom.NumberOfMetrics; i++ {
					metrics = append(metrics, struct{}{})
				}
				resp = map[string]interface{}{
					"data": metrics,
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

func assertGetOrgServiceStatus(t *testing.T, user *users.User, org *users.Organization, cfg *mockServicesConfig, now interface{}) {
	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/status", nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Equal(t, map[string]interface{}{
		"connected": (cfg.Flux.Connected ||
			cfg.Scope.NumberOfProbes > 0 ||
			cfg.Prom.NumberOfMetrics > 0 ||
			cfg.Net.NumberOfPeers > 0),
		"firstSeenConnectedAt": now,
		"flux": map[string]interface{}{
			"fluxsvc": map[string]interface{}{},
			"fluxd": map[string]interface{}{
				"connected": cfg.Flux.Connected,
			},
			"git": map[string]interface{}{
				"configured": false,
				"config":     nil,
			},
		},
		"scope": map[string]interface{}{
			"numberOfProbes": float64(cfg.Scope.NumberOfProbes),
		},
		"prom": map[string]interface{}{
			"numberOfMetrics": float64(cfg.Prom.NumberOfMetrics),
		},
		"net": map[string]interface{}{
			"numberOfPeers": float64(cfg.Net.NumberOfPeers),
		},
	}, body)
}

func Test_GetOrgServiceStatus(t *testing.T) {
	now := time.Date(2017, 1, 1, 1, 1, 0, 0, time.UTC)
	cfg := &mockServicesConfig{}
	mockServices := MockServices(cfg)
	setupWithMockServices(t,
		mockServices.URL+"/api/flux/v6/status",
		mockServices.URL+"/api/probes",
		mockServices.URL+"/api/prom/api/v1/label/__name__/values",
		mockServices.URL+"/api/net/peer",
	)
	defer cleanup(t)

	user, org := getOrg(t)
	cfg.AcceptedOrgID = org.ID
	body := map[string]interface{}{}

	// Test when services are down.
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/status", nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"connected":            false,
			"firstSeenConnectedAt": nil,
			"flux": map[string]interface{}{
				"fluxsvc": map[string]interface{}{},
				"fluxd": map[string]interface{}{
					"connected": false,
				},
				"git": map[string]interface{}{
					"configured": false,
					"config":     nil,
				},
				"error": "Unexpected status code: 500",
			},
			"scope": map[string]interface{}{
				"numberOfProbes": float64(0),
				"error":          "Unexpected status code: 500",
			},
			"prom": map[string]interface{}{
				"numberOfMetrics": float64(0),
				"error":           "Unexpected status code: 500",
			},
			"net": map[string]interface{}{
				"numberOfPeers": float64(0),
				"error":         "Unexpected status code: 500",
			},
		}, body)
	}

	// Test when services are online but not connected.
	{
		cfg.Flux.Online = true
		cfg.Scope.Online = true
		cfg.Prom.Online = true
		cfg.Net.Online = true

		assertGetOrgServiceStatus(t, user, org, cfg, nil)
	}

	// Test when services are online and connected.
	{
		cfg.Flux.Connected = true
		cfg.Scope.NumberOfProbes = 3
		cfg.Prom.NumberOfMetrics = 4
		cfg.Net.NumberOfPeers = 2

		mtime.NowForce(now)
		assertGetOrgServiceStatus(t, user, org, cfg, now.Format(time.RFC3339))
		mtime.NowReset()
	}

	// Test when services are then disconnected.
	{
		cfg.Flux.Connected = false
		cfg.Scope.NumberOfProbes = 0
		cfg.Prom.NumberOfMetrics = 0
		cfg.Net.NumberOfPeers = 0

		assertGetOrgServiceStatus(t, user, org, cfg, now.Format(time.RFC3339))
	}
}
