package zuora_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/billing/zuora"
)

func TestToZuoraAccountNumber(t *testing.T) {
	assert.Equal(t, "W07a5fd8c403976ced4e81b7da61f31d", zuora.ToZuoraAccountNumber("foo-moo-99"))
}
