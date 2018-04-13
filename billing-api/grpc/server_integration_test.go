//+build integration

package grpc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db/dbtest"
	commongrpc "github.com/weaveworks/service/common/billing/grpc"
)

func TestFindBillingAccountByTeamID_NotFound(t *testing.T) {
	grpcPort := 9097
	database := dbtest.Setup(t)
	go startNewServer(t, grpcPort, database)

	client := newClient(t, fmt.Sprintf("localhost:%v", grpcPort))
	defer client.Close()

	<-ready // Wait for the server to be ready.

	billingAccount, err := client.FindBillingAccountByTeamID(context.Background(), &commongrpc.BillingAccountByTeamIDRequest{
		TeamID: "456",
	})
	assert.NoError(t, err)
	assert.Equal(t, &commongrpc.BillingAccount{}, billingAccount)
}
