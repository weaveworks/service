package attrsync

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/segmentio/analytics-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/weaveworks/common/logging"

	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	users_db "github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
)

type MockSegment struct {
	Messages []analytics.Message
}

func (m *MockSegment) Enqueue(msg analytics.Message) error {
	m.Messages = append(m.Messages, msg)
	return nil
}

func (m *MockSegment) Close() error {
	return nil
}

type testFixtures struct {
	billingClient *billing_grpc.MockBillingClient
	ctrl          *gomock.Controller
	ctx           context.Context
	db            users_db.DB
	mockSegment   *MockSegment
}

func setup(t *testing.T) (testFixtures, *AttributeSyncer) {
	ctx := context.Background()
	db := dbtest.Setup(t)

	logger := logging.Logrus(logrus.StandardLogger())

	ctrl := gomock.NewController(t)
	billingClient := billing_grpc.NewMockBillingClient(ctrl)
	mockSegment := MockSegment{}

	attrSync := New(logger, db, billingClient, &mockSegment)

	return testFixtures{billingClient, ctrl, ctx, db, &mockSegment}, attrSync
}

func (tf *testFixtures) cleanup(t *testing.T) {
	dbtest.Cleanup(t, tf.db)
	tf.ctrl.Finish()
}

func Test_AttrSyncNoOrgs(t *testing.T) {
	tf, attrSync := setup(t)
	defer tf.cleanup(t)

	user := dbtest.GetUser(t, tf.db)
	require.NoError(t, attrSync.syncUser(tf.ctx, user))
}

func Test_AttrSyncWithOrg(t *testing.T) {
	tf, attrSync := setup(t)
	defer tf.cleanup(t)

	user, org := dbtest.GetOrg(t, tf.db)
	user = dbtest.AddUserInfoToUser(t, tf.db, user)

	now := time.Now()
	tf.db.SetOrganizationFirstSeenConnectedAt(
		tf.ctx, org.ExternalID, &now)
	tf.db.SetOrganizationFirstSeenPromConnectedAt(
		tf.ctx, org.ExternalID, &now)

	tf.billingClient.EXPECT().GetInstanceBillingStatus(
		gomock.Any(), &billing_grpc.InstanceBillingStatusRequest{
			InternalID: org.ID,
		}).
		Times(1).
		Return(&billing_grpc.InstanceBillingStatusResponse{
			BillingStatus: billing_grpc.ACTIVE,
		}, nil)

	require.NoError(t, attrSync.syncUser(tf.ctx, user))

	require.Len(t, tf.mockSegment.Messages, 1)
	ident, ok := tf.mockSegment.Messages[0].(analytics.Identify)
	require.True(t, ok)

	require.Equal(t, ident, analytics.Identify{
		UserId: user.Email,
		Traits: analytics.Traits{
			"name":  user.Name,
			"email": user.Email,
			"company": map[string]string{
				"name": user.Company,
			},

			"instances_ever_connected_flux_total":          0,
			"instances_ever_connected_net_total":           0,
			"instances_ever_connected_prom_total":          1,
			"instances_ever_connected_scope_total":         0,
			"instances_ever_connected_total":               1,
			"instances_status_active_total":                1,
			"instances_status_payment_due_total":           0,
			"instances_status_payment_error_total":         0,
			"instances_status_subscription_inactive_total": 0,
			"instances_status_trial_active_total":          0,
			"instances_status_trial_expired_total":         0,
			"instances_status_unknown_total":               0,
			"instances_total":                              1,
		},
	})
}
