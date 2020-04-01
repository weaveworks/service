package balance

import (
	"strconv"
	"testing"

	"github.com/go-kit/kit/sd"
	"github.com/stretchr/testify/assert"
)

// e dummy implementation of Endpoint for testing.
type e string

// Override the hash function to return easier to reason about values. Assumes
// the keys can be converted to an integer.
func testHash(key []byte) uint32 {
	i, err := strconv.Atoi(string(key))
	if err != nil {
		panic(err)
	}
	return uint32(i)
}

func event(endpoints ...string) sd.Event {
	ev := sd.Event{}
	for _, e := range endpoints {
		ev.Instances = append(ev.Instances, e)
	}
	return ev
}

func TestEndpointUpdates(t *testing.T) {
	cw := &consistentWrapper{
		cache: make(map[string]Endpoint),
		c: NewConsistent(ConsistentConfig{
			Hash:       testHash,
			LoadFactor: 1.25,
		}),
	}

	_, err := cw.Get("11") // Should error as it has no endpoints
	assert.Error(t, err)
	cw.update(event("4"))
	assert.Equal(t, 1, cw.c.numEndpoints)
	cw.update(event("6", "4", "2"))
	assert.Equal(t, 3, cw.c.numEndpoints)
	endpoint2, err := cw.Get("11")
	assert.NoError(t, err)
	assert.Equal(t, "2", endpoint2.Key())
	endpoint4, err := cw.Get("33")
	assert.NoError(t, err)
	assert.Equal(t, "4", endpoint4.Key())
	assert.Equal(t, 2, cw.c.totalLoad)

	// Remove endpoints that had requests pending. The load should be
	// adjusted and Put() be a no op.
	cw.update(event("6"))
	assert.Equal(t, 1, cw.c.numEndpoints)
	assert.Equal(t, 0, cw.c.totalLoad)
	cw.Put(endpoint2)
	cw.Put(endpoint4)
}
