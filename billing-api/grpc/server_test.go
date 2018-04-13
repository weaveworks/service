package grpc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db/mock_db"
	commongrpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"golang.org/x/net/context"
)

func TestFindBillingAccountByTeamID_AgainstMockDB(t *testing.T) {
	grpcPort := 9096
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	database := mock_db.NewMockDB(ctrl)
	teamID := "123"
	database.EXPECT().FindBillingAccountByTeamID(gomock.Any(), teamID).Return(expectedBillingAccount(), nil)
	go startNewServer(t, grpcPort, database)

	client := newClient(t, fmt.Sprintf("localhost:%v", grpcPort))
	defer client.Close()

	<-ready // Wait for the server to be ready.

	billingAccount, err := client.FindBillingAccountByTeamID(context.Background(), &commongrpc.BillingAccountByTeamIDRequest{
		TeamID: teamID,
	})
	assert.NoError(t, err)
	assert.Equal(t, expectedBillingAccount(), billingAccount)
}

func expectedBillingAccount() *commongrpc.BillingAccount {
	return &commongrpc.BillingAccount{
		ID:        1,
		CreatedAt: time.Date(2018, 04, 13, 0, 0, 0, 0, time.UTC),
		Provider:  provider.External,
	}
}
