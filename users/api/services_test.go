package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/common/mtime"
)

type mockServicesConfig struct {
	Flux struct {
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
			if config.Net.Online {
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

func Test_GetOrgStatus(t *testing.T) {
	now := time.Now()
	cfg := &mockServicesConfig{}
	mockServices := MockServices(cfg)
	setupWithMockServices(t, mockServices.URL)
	defer cleanup(t)

	user, org := getOrg(t)
	body := map[string]interface{}{}

	// Test when services are down.
	{
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/status", nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"connected":        false,
			"firstConnectedAt": nil,
			"flux": map[string]interface{}{
				"fluxsvc": map[string]interface{}{},
				"fluxd": map[string]interface{}{
					"connected": false,
					"last":      "0001-01-01T00:00:00Z",
				},
				"git": map[string]interface{}{
					"configured": false,
					"config":     nil,
				},
				"error": "Could not decode flux data",
			},
			"scope": map[string]interface{}{
				"numberOfProbes": float64(0),
				"error":          "Could not decode scope data",
			},
			"prom": map[string]interface{}{
				"numberOfMetrics": float64(0),
				"error":           "Could not decode prom data",
			},
			"net": map[string]interface{}{
				"numberOfPeers": float64(0),
				"error":         "Could not decode net data",
			},
		}, body)
	}

	// Test when services are online but not connected.
	{
		cfg.Flux.Online = true
		cfg.Scope.Online = true
		cfg.Prom.Online = true
		cfg.Net.Online = true

		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/status", nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"connected":        false,
			"firstConnectedAt": nil,
			"flux": map[string]interface{}{
				"fluxsvc": map[string]interface{}{},
				"fluxd": map[string]interface{}{
					"connected": false,
					"last":      "0001-01-01T00:00:00Z",
				},
				"git": map[string]interface{}{
					"configured": false,
					"config":     nil,
				},
			},
			"scope": map[string]interface{}{
				"numberOfProbes": float64(0),
			},
			"prom": map[string]interface{}{
				"numberOfMetrics": float64(0),
			},
			"net": map[string]interface{}{
				"numberOfPeers": float64(0),
			},
		}, body)

		mtime.NowReset()
	}

	// Test when services are online and connected.
	{
		cfg.Flux.Connected = true
		cfg.Scope.NumberOfProbes = 3
		cfg.Prom.NumberOfMetrics = 4
		cfg.Net.NumberOfPeers = 2

		mtime.NowForce(now)
		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/status", nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"connected":        true,
			"firstConnectedAt": now.Format("2006-01-02T15:04:05.000000000-07:00"),
			"flux": map[string]interface{}{
				"fluxsvc": map[string]interface{}{},
				"fluxd": map[string]interface{}{
					"connected": true,
					"last":      "0001-01-01T00:00:00Z",
				},
				"git": map[string]interface{}{
					"configured": false,
					"config":     nil,
				},
			},
			"scope": map[string]interface{}{
				"numberOfProbes": float64(3),
			},
			"prom": map[string]interface{}{
				"numberOfMetrics": float64(4),
			},
			"net": map[string]interface{}{
				"numberOfPeers": float64(2),
			},
		}, body)

		mtime.NowReset()
	}

	// Test when services are then disconnected.
	{
		cfg.Flux.Connected = false
		cfg.Scope.NumberOfProbes = 0
		cfg.Prom.NumberOfMetrics = 0
		cfg.Net.NumberOfPeers = 0

		w := httptest.NewRecorder()
		r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID+"/status", nil)

		app.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, map[string]interface{}{
			"connected":        false, // Now false but firstConnectedAt still set.
			"firstConnectedAt": now.Format("2006-01-02T15:04:05.000000000-07:00"),
			"flux": map[string]interface{}{
				"fluxsvc": map[string]interface{}{},
				"fluxd": map[string]interface{}{
					"connected": false,
					"last":      "0001-01-01T00:00:00Z",
				},
				"git": map[string]interface{}{
					"configured": false,
					"config":     nil,
				},
			},
			"scope": map[string]interface{}{
				"numberOfProbes": float64(0),
			},
			"prom": map[string]interface{}{
				"numberOfMetrics": float64(0),
			},
			"net": map[string]interface{}{
				"numberOfPeers": float64(0),
			},
		}, body)
	}
}
