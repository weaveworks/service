package storage

import "github.com/weaveworks/service/users"

func newUsersOrganizationFilter(s []string) Filter {
	return And{
		InFilter{
			SQLField: "organizations.external_id",
			SQLJoins: []string{
				"memberships on (memberships.user_id = users.id)",
				"organizations on (memberships.organization_id = organizations.id)",
			},
			Value: s,
			Allowed: func(i interface{}) bool {
				if u, ok := i.(*users.User); ok {
					for _, org := range u.Organizations {
						for _, externalID := range s {
							if org.ExternalID == externalID {
								return true
							}
						}
					}
				}
				return false
			},
		},
		InFilter{
			SQLField: "memberships.deleted_at",
			Value:    nil,
			Allowed:  func(i interface{}) bool { return true },
		},
	}
}
