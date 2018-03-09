package orgs

import (
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
)

const (
	// RefuseDataAccess disables access to Weave Cloud for users of a given organisation.
	RefuseDataAccess = "RefuseDataAccess"
	// RefuseDataUpload disables ingestion by Weave Cloud of new data for a given organisation.
	RefuseDataUpload = "RefuseDataUpload"
)

// DelinquentFilter filters an organization that is supposed to pay
// but has no payment method set up.
func DelinquentFilter(now time.Time) filter.Organization {
	return filter.And(
		filter.ZuoraAccount(false),
		filter.GCP(false),
		filter.TrialExpiredBy(now),
		filter.HasFeatureFlag(users.BillingFeatureFlag),
	)
}

func isDelinquent(o users.Organization, now time.Time) bool {
	return DelinquentFilter(now).MatchesOrg(o)

}

// ShouldRefuseDataAccess returns true if the organization's flag is supposed to be set.
func ShouldRefuseDataAccess(o users.Organization, now time.Time) bool {
	return isDelinquent(o, now)
}

// ShouldRefuseDataUpload returns true if the organization's flag is supposed to be set.
func ShouldRefuseDataUpload(o users.Organization, now time.Time) bool {
	// At the time we introduced this automatic data upload block, we didn't want to block
	// access *and* upload at the same time for any organization. For this reason, we will
	// only start blocking upload 15days from today.
	// TODO: remove this bit after 2018-03-21
	if now.Before(time.Date(2018, 03, 21, 0, 0, 0, 0, time.UTC)) {
		return false
	}

	return isDelinquent(o, now) && o.TrialExpiresAt.Add(users.TrialRefuseDataUploadAfter).Before(now)
}
