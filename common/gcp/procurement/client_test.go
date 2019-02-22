package procurement_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/service/common/gcp/procurement"
)

func TestEntitlement_AccountID(t *testing.T) {
	e := procurement.Entitlement{Account: "providers/weaveworks-dev/accounts/E-123"}
	assert.Equal(t, "E-123", e.AccountID())
}
