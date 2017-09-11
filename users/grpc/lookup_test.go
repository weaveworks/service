package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/db"
	"github.com/weaveworks/service/users/db/dbtest"
	"github.com/weaveworks/service/users/sessions"
)

var (
	database     db.DB
	sessionStore sessions.Store
	server       users.UsersServer
	ctx          context.Context
)

func setup(t *testing.T) {
	logging.Setup("debug")
	database = dbtest.Setup(t)
	sessionStore = sessions.MustNewStore("Test-Session-Secret-Which-Is-64-Bytes-Long-aa1a166556cb719f531cd", false)
	server = New(sessionStore, database)
	ctx = context.Background()
}

func cleanup(t *testing.T) {
	dbtest.Cleanup(t, database)
}

func Test_SetOrganizationFlag(t *testing.T) {
	setup(t)
	defer cleanup(t)
	_, org := dbtest.GetOrg(t, database)

	assert.False(t, org.DenyUIFeatures)
	assert.False(t, org.DenyTokenAuth)

	server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "DenyUIFeatures",
			Value:      true,
		})
	resp, _ := server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	require.True(t, resp.Organization.DenyUIFeatures)

	server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "DenyTokenAuth",
			Value:      true,
		})
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	require.True(t, resp.Organization.DenyTokenAuth)

	server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "",
			Value:      true,
		})
}

func Test_SetOrganizationAccountNumber(t *testing.T) {
	setup(t)
	defer cleanup(t)
	_, org := dbtest.GetOrg(t, database)

	// initial state
	assert.Nil(t, org.ZuoraAccountCreatedAt)
	assert.Empty(t, org.ZuoraAccountNumber)

	// set number, automatically sets createdAt
	server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     "Wfirst-set",
		})
	ts := time.Now()
	resp, _ := server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	assert.Equal(t, "Wfirst-set", resp.Organization.ZuoraAccountNumber)
	assert.True(t, resp.Organization.ZuoraAccountCreatedAt.Before(ts))

	// update number, also updates createdAt
	server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     "Wupdate",
		})
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	assert.Equal(t, "Wupdate", resp.Organization.ZuoraAccountNumber)
	assert.True(t, resp.Organization.ZuoraAccountCreatedAt.After(ts))

	// explicitly set createdAt
	createdAt := time.Now()
	server.SetOrganizationZuoraAccount(
		ctx, &users.SetOrganizationZuoraAccountRequest{
			ExternalID: org.ExternalID,
			Number:     "Wexplicit-date",
			CreatedAt:  &createdAt,
		})
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	assert.Equal(t, "Wexplicit-date", resp.Organization.ZuoraAccountNumber)
	assert.True(t, resp.Organization.ZuoraAccountCreatedAt.Equal(createdAt))
}
