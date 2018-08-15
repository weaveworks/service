package attrsync

import (
	"context"
	"fmt"
	"strings"
	"time"

	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/users"
)

const orgsAttrPrefix = "instances_"

func (c *AttributeSyncer) userOrgAttributes(ctx context.Context, user *users.User) (map[string]int, error) {
	orgs, err := c.db.ListOrganizationsForUserIDs(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	attrs := map[string]int{
		orgsAttrPrefix + "total": len(orgs),
	}
	addAttrs := func(toAdd map[string]int) {
		for k, v := range toAdd {
			attrs[k] = v
		}
	}
	billingStatusAttrs, err := c.userOrgBillingStatus(ctx, orgs)
	if err == nil {
		addAttrs(billingStatusAttrs)
	} else {
		c.log.WithField("err", err).Warnln("Error querying billing status. Skipping attributes.")
	}
	addAttrs(c.userOrgOnboardingStatus(ctx, orgs))

	return attrs, nil
}

func (c *AttributeSyncer) userOrgBillingStatus(ctx context.Context, orgs []*users.Organization) (map[string]int, error) {
	attributeNames := map[billing_grpc.BillingStatus]string{}
	orgBillingStatusCount := map[string]int{}
	for statusInt, name := range billing_grpc.BillingStatus_name {
		status := billing_grpc.BillingStatus(statusInt)
		attributeName := fmt.Sprintf(orgsAttrPrefix+"status_%s_total", strings.ToLower(name))
		attributeNames[status] = attributeName
		orgBillingStatusCount[attributeName] = 0
	}
	for _, org := range orgs {
		resp, err := c.billingClient.GetInstanceBillingStatus(
			ctx, &billing_grpc.InstanceBillingStatusRequest{InternalID: org.ID},
		)
		if err == nil {
			orgBillingStatusCount[attributeNames[resp.BillingStatus]]++
		}
	}

	return orgBillingStatusCount, nil
}

func (c *AttributeSyncer) userOrgOnboardingStatus(ctx context.Context, orgs []*users.Organization) map[string]int {
	orgAgentFirstSeenCount := map[string]int{
		orgsAttrPrefix + "ever_connected_total":       0,
		orgsAttrPrefix + "ever_connected_flux_total":  0,
		orgsAttrPrefix + "ever_connected_prom_total":  0,
		orgsAttrPrefix + "ever_connected_net_total":   0,
		orgsAttrPrefix + "ever_connected_scope_total": 0,
	}

	incr := func(key string, seen *time.Time) {
		if seen != nil {
			orgAgentFirstSeenCount[key]++
		}
	}

	for _, org := range orgs {
		incr(orgsAttrPrefix+"ever_connected_total", org.FirstSeenConnectedAt)
		incr(orgsAttrPrefix+"ever_connected_flux_total", org.FirstSeenFluxConnectedAt)
		incr(orgsAttrPrefix+"ever_connected_prom_total", org.FirstSeenPromConnectedAt)
		incr(orgsAttrPrefix+"ever_connected_net_total", org.FirstSeenNetConnectedAt)
		incr(orgsAttrPrefix+"ever_connected_scope_total", org.FirstSeenScopeConnectedAt)
	}
	return orgAgentFirstSeenCount
}
