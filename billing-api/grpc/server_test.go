package grpc_test

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/billing-api/db/mock_db"
	common_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/billing/provider"
	"golang.org/x/net/context"
)

func TestFindBillingAccountByTeamID_AgainstMockDB(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	database := mock_db.NewMockDB(ctrl)
	teamID := "123"
	database.EXPECT().FindBillingAccountByTeamID(gomock.Any(), teamID).Return(expectedBillingAccount(), nil)

	listener := startNewServer(t, database)
	client := newClient(t, listener.Addr().String())
	defer client.Close()

	billingAccount, err := client.FindBillingAccountByTeamID(context.Background(), &common_grpc.BillingAccountByTeamIDRequest{
		TeamID: teamID,
	})
	assert.NoError(t, err)
	assert.Equal(t, expectedBillingAccount(), billingAccount)
}

func expectedBillingAccount() *common_grpc.BillingAccount {
	return &common_grpc.BillingAccount{
		ID:        1,
		CreatedAt: time.Date(2018, 04, 13, 0, 0, 0, 0, time.UTC),
		Provider:  provider.External,
	}
}
