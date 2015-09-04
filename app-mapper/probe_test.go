package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
)

type probeStorage interface {
	probeBumper
	probeGetter
}

func TestProbeStorage(t *testing.T) {
	const (
		orgID   = "somePrivateOrgID"
		probeID = "someProbeID"
	)

	var probeStorage probeStorage
	if isIntegrationTest {
		db, err := sqlx.Open("postgres", defaultDBURI)
		require.NoError(t, err, "Cannot initialize database client")
		probeStorage = newProbeDBStorage(db)
	} else {
		probeStorage = &probeMemStorage{}
	}
	t1 := time.Now()
	err := probeStorage.bumpProbeLastSeen(probeID, orgID)
	require.NoError(t, err, "bumpProbeLastSeen unexpectedly failed")
	t2 := time.Now()

	probes, err := probeStorage.getProbesFromOrg(orgID)
	require.NoError(t, err, "getProbesFromOrg unexpectedly failed")
	require.Equal(t, 1, len(probes), "unexpected probes length")
	assert.Equal(t, probeID, probes[0].ID)
	assert.WithinDuration(t, t1, probes[0].LastSeen, t2.Sub(t1))

	// Bump a second time
	t1 = time.Now()
	err = probeStorage.bumpProbeLastSeen(probeID, orgID)
	require.NoError(t, err, "bumpProbeLastSeen unexpectedly failed")
	t2 = time.Now()

	probes, err = probeStorage.getProbesFromOrg(orgID)
	require.NoError(t, err, "getProbesFromOrg unexpectedly failed")
	require.Equal(t, 1, len(probes), "unexpected probes length")
	assert.Equal(t, probeID, probes[0].ID)
	assert.WithinDuration(t, t1, probes[0].LastSeen, t2.Sub(t1))
}

func TestProbeObserver(t *testing.T) {
	const (
		orgName = "somePublicOrgName"
		orgID   = "someInternalOrgID"
	)
	expectedResult := []probe{
		{"someProbeID1", time.Now().UTC()},
		{"someProbeID2", time.Now().UTC()},
	}
	p := &probeMemStorage{[]memProbe{
		{expectedResult[0], orgID},
		{expectedResult[1], orgID},
	}}

	var recordedOrgName string
	a := authenticatorFunc(func(r *http.Request, orgName string) (authenticatorResponse, error) {
		recordedOrgName = orgName
		return authenticatorResponse{orgID}, nil
	})

	router := mux.NewRouter()
	newProbeObserver(a, p).registerHandlers(router)
	req, err := http.NewRequest("GET", "http://example.com/api/org/"+orgName+"/probes", nil)
	require.NoError(t, err, "Cannot create request")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	var result []probe
	require.Equal(t, http.StatusOK, w.Code, "Request unexpectedly failed")
	err = json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err, "Cannot decode result")
	assert.Equal(t, expectedResult, result, "Unexpected result")
	assert.Equal(t, orgName, recordedOrgName, "Organization name not correctly forward to authenticator")
}
