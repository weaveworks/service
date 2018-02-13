package main_test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/weaveworks/common/instrument"
	"github.com/weaveworks/service/billing-enforcer/job"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

func TestNotifyTrialOrganizations(t *testing.T) {
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

func TestNotifyDelinquentOrganizations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	now := time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)
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

	j := job.NewEnforce(client, job.Config{}, instrument.NewJobCollector("foo"))
	j.NotifyDelinquentOrganizations(context.Background(), now)
}
