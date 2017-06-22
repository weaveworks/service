package grpc

import (
	"context"
	"testing"

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

	_, err := server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "DenyUIFeatures",
			Value:      true,
		})
	require.NoError(t, err)
	resp, _ := server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	require.True(t, resp.Organization.DenyUIFeatures)

	_, err = server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "DenyTokenAuth",
			Value:      true,
		})
	require.NoError(t, err)
	resp, _ = server.GetOrganization(ctx, &users.GetOrganizationRequest{ExternalID: org.ExternalID})
	require.True(t, resp.Organization.DenyTokenAuth)

	_, err = server.SetOrganizationFlag(
		ctx, &users.SetOrganizationFlagRequest{
			ExternalID: org.ExternalID,
			Flag:       "",
			Value:      true,
		})
	require.Error(t, err)
}
