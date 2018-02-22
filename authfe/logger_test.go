package main

import (
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/mock_users"
)

func TestNewLauncherServiceLogger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	u := mock_users.NewMockUsersClient(ctrl)
	u.EXPECT().
		GetOrganization(gomock.Any(), &users.GetOrganizationRequest{
			ID: &users.GetOrganizationRequest_ExternalID{ExternalID: "external-id-1"},
		}).
		Return(&users.GetOrganizationResponse{
			Organization: users.Organization{ID: "2"},
		}, nil)

	logger := newLauncherServiceLogger(u)
	req, err := http.NewRequest("GET", "https://get.weave.works/k8s/example.yaml?foo=1&instanceID=external-id-1", nil)
	assert.NoError(t, err)

	event, success := logger(req)
	assert.Equal(t, event, Event{
		ID:             "/k8s/example.yaml",
		Product:        "launcher-service",
		UserAgent:      "",
		OrganizationID: "2",
		IPAddress:      "",
	})
	assert.Equal(t, success, true)
}
