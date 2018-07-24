package reporter_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/scope/report"
	s_reflect "github.com/weaveworks/scope/test/reflect"

	"github.com/weaveworks/service/billing-synthetic-usage-injector/pkg/injector/scope/reporter"
)

const maxFakeHosts = 128

func TestSerialiseDeserialise(t *testing.T) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	numFakeHosts := uint(rand.Int31n(maxFakeHosts))
	r := reporter.NewFakeScopeReporter(numFakeHosts)
	r1, err := r.Report()
	assert.NoError(t, err)
	assert.Len(t, r1.Host.Nodes, int(numFakeHosts))

	buf, err := r1.WriteBinary()
	assert.NoError(t, err)

	r2, err := report.MakeFromBytes(buf.Bytes())
	assert.NoError(t, err)
	assert.True(t, s_reflect.DeepEqual(r1, *r2), "%v != %v", r1, *r2)

	r3, err := report.MakeFromBinary(buf)
	assert.NoError(t, err)
	assert.True(t, s_reflect.DeepEqual(r1, *r3), "%v != %v", r1, *r3)
}
