package permissions

import (
	"context"

	"github.com/weaveworks/service/users/db"
)

// TeamMemberUpdate permission allows granting/removing permissions
const TeamMemberUpdate = "team.member.update"

// CanInviteTeamMembers allows inviting new team members
func CanInviteTeamMembers(ctx context.Context, d db.DB, userID, orgExternalID string) (bool, error) {
	return HasUserOrgPermissionTo(ctx, d, userID, orgExternalID, "team.member.invite")
}

// InstanceDelete permission allows deleting team instances
const InstanceDelete = "instance.delete"

// InstanceBillingUpdate permission allows updating billing information
const InstanceBillingUpdate = "instance.billing.update"

// AlertSettingsUpdate permission allows editing alerting rules
const AlertSettingsUpdate = "alert.settings.update"

// TODO: func CanManageTeamMembers(user, team)
// TODO: func CanDeleteInstance(user, org)
// TODO: func CanUpdateBillingInfo(user, org)
// TODO: func CanUpdateAlertSettings(user, org)
