package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db/filter"
)

const expiryGracePeriod = time.Hour * 24 * 7 * 2 // two weeks

func (a *usersServer) GetDataExpiry(ctx context.Context, req *users.DataExpiryRequest) (*users.DataExpiryResponse, error) {
	if req.OrganizationID != "" {
		organizations, err := a.db.ListAllOrganizations(ctx, filter.ID(req.OrganizationID), "", 0)
		if err != nil {
			return nil, err
		} else if len(organizations) == 0 {
			return nil, users.ErrNotFound
		} else if len(organizations) != 1 {
			return nil, errors.New("internal error: multiple ID matches")
		}
		expiryDate := expiryDate(organizations[0])
		return &users.DataExpiryResponse{ExpireBefore: &expiryDate}, nil
	}

	return nil, nil
}

func expiryDate(org *users.Organization) time.Time {
	if !org.DeletedAt.IsZero() {
		return org.DeletedAt.Add(expiryGracePeriod)
	}
	if org.RefuseDataAccess && org.RefuseDataUpload && !org.TrialExpiresAt.IsZero() {
		return org.TrialExpiresAt.Add(expiryGracePeriod)
	}

	return time.Time{} // no time - do not delete
}
