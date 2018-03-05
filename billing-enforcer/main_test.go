package main_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing-enforcer/job"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

func TestEnforce_NotifyTrialOrganizations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	now := time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetTrialOrganizations(ctx, &users.GetTrialOrganizationsRequest{Now: now}).
		Return(&users.GetTrialOrganizationsResponse{
			Organizations: []users.Organization{
				{
					ExternalID:                   "notify-yes",
					TrialExpiresAt:               now.Add(1 * 24 * time.Hour),
					TrialPendingExpiryNotifiedAt: nil,
					FirstSeenConnectedAt:         &now,
				},
				{ // already notified
					ExternalID:                   "notify-no-already",
					TrialExpiresAt:               now.Add(1 * 24 * time.Hour),
					TrialPendingExpiryNotifiedAt: &now, // any date
					FirstSeenConnectedAt:         &now,
				},
				{ // is not yet within notification range
					ExternalID:                   "notify-no-notyet",
					TrialExpiresAt:               now.Add(6 * 24 * time.Hour),
					TrialPendingExpiryNotifiedAt: nil,
					FirstSeenConnectedAt:         &now,
				},
				{ // not onboarded
					ExternalID:                   "notify-no-not-onboarded",
					TrialExpiresAt:               now.Add(1 * 24 * time.Hour),
					TrialPendingExpiryNotifiedAt: nil,
					FirstSeenConnectedAt:         nil,
				},
			},
		}, nil)

	client.EXPECT().
		NotifyTrialPendingExpiry(ctx, &users.NotifyTrialPendingExpiryRequest{ExternalID: "notify-yes"})

	j := job.NewEnforce(client, job.Config{NotifyPendingExpiryPeriod: 5 * 24 * time.Hour},
		instrument.NewJobCollector("foo"))
	j.NotifyTrialOrganizations(context.Background(), now)
}

func TestEnforce_ProcessDelinquentOrganizations_notifyTrialExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	now := time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now}).
		Return(&users.GetDelinquentOrganizationsResponse{
			Organizations: []users.Organization{
				{
					ExternalID:             "notify-yes",
					TrialExpiredNotifiedAt: nil,
					FirstSeenConnectedAt:   &now,
				},
				{ // already notified
					ExternalID:             "notify-no-already",
					TrialExpiredNotifiedAt: &now,
					FirstSeenConnectedAt:   &now,
				},
				{
					ExternalID:             "notify-no-not-onboarded",
					TrialExpiredNotifiedAt: nil,
					FirstSeenConnectedAt:   nil,
				},
			},
		}, nil)

	client.EXPECT().
		NotifyTrialExpired(ctx, &users.NotifyTrialExpiredRequest{ExternalID: "notify-yes"})

	client.EXPECT().
		SetOrganizationFlag(ctx, gomock.Any()).
		AnyTimes()

	j := job.NewEnforce(client, job.Config{}, instrument.NewJobCollector("foo"))
	j.ProcessDelinquentOrganizations(context.Background(), now)
}

func TestEnforce_ProcessDelinquentOrganizations_refuseData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	now := time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)
	expiredAccess := now.Add(-2 * 24 * time.Hour)
	expiredUpload := now.Add(-17 * 24 * time.Hour)
	client := mock_users.NewMockUsersClient(ctrl)
	client.EXPECT().
		GetDelinquentOrganizations(ctx, &users.GetDelinquentOrganizationsRequest{Now: now}).
		Return(&users.GetDelinquentOrganizationsResponse{
			Organizations: []users.Organization{
				{
					ExternalID:             "refuse-access",
					TrialExpiredNotifiedAt: &now,
					FirstSeenConnectedAt:   &now,

					TrialExpiresAt:   expiredAccess,
					RefuseDataAccess: false,
					RefuseDataUpload: false,
				},
				{
					ExternalID:             "refuse-access-already",
					TrialExpiredNotifiedAt: &now,
					FirstSeenConnectedAt:   &now,

					TrialExpiresAt:   expiredAccess,
					RefuseDataAccess: true,
					RefuseDataUpload: false,
				},
				{
					ExternalID:             "refuse-upload",
					TrialExpiredNotifiedAt: &now,
					FirstSeenConnectedAt:   &now,

					TrialExpiresAt:   expiredUpload,
					RefuseDataAccess: true,
					RefuseDataUpload: false,
				},
				{
					ExternalID:             "refuse-upload-already",
					TrialExpiredNotifiedAt: &now,
					FirstSeenConnectedAt:   &now,

					TrialExpiresAt:   expiredUpload,
					RefuseDataAccess: true,
					RefuseDataUpload: true,
				},
			},
		}, nil)

	client.EXPECT().
		SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
			ExternalID: "refuse-access",
			Flag:       orgs.RefuseDataAccess,
			Value:      true,
		})

	client.EXPECT().
		SetOrganizationFlag(ctx, &users.SetOrganizationFlagRequest{
			ExternalID: "refuse-upload",
			Flag:       orgs.RefuseDataUpload,
			Value:      true,
		})
	client.EXPECT().
		NotifyRefuseDataUpload(ctx, &users.NotifyRefuseDataUploadRequest{ExternalID: "refuse-upload"})

	j := job.NewEnforce(client, job.Config{RefuseDataUploadAfter: 15 * 24 * time.Hour}, instrument.NewJobCollector("foo"))
	j.ProcessDelinquentOrganizations(context.Background(), now)
}
