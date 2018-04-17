//+build integration

package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db/dbtest"
	common_grpc "github.com/weaveworks/service/common/billing/grpc"
)

func TestFindBillingAccountByTeamID_NotFound(t *testing.T) {
	database := dbtest.Setup(t)
	listener := startNewServer(t, database)
	client := newClient(t, listener.Addr().String())
	defer client.Close()

	billingAccount, err := client.FindBillingAccountByTeamID(context.Background(), &common_grpc.BillingAccountByTeamIDRequest{
		TeamID: "456",
	})
	assert.NoError(t, err)
	assert.Equal(t, &common_grpc.BillingAccount{}, billingAccount)
}
