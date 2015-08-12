// +build integration

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeDBStorage(t *testing.T) {
	const (
		orgID   = "somePrivateOrgID"
		probeID = "someProbeID"
	)
	p := newProbeDBStorage(testDB)
	t1 := time.Now()
	err := p.bumpProbeLastSeen(probeID, orgID)
	require.NoError(t, err, "bumpProbeLastSeen unexpectedly failed")
	t2 := time.Now()

	probes, err := p.getProbesFromOrg(orgID)
	require.NoError(t, err, "getProbesFromOrg unexpectedly failed")
	require.Equal(t, 1, len(probes), "unexpected probes length")
	assert.Equal(t, probeID, probes[0].ID)
	assert.Equal(t, orgID, probes[0].OrgID)
	assert.WithinDuration(t, t1, probes[0].LastSeen, t2.Sub(t1))
}
