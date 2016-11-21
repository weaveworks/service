package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/service/configs"
	"github.com/weaveworks/service/configs/api"
)

// The root page returns 200 OK.
func Test_Root_OK(t *testing.T) {
	setup(t)
	defer cleanup(t)

	w := request(t, "GET", "/", nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

// configs returns 401 to requests without authentication.
func Test_GetUserConfig_Anonymous(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem := makeSubsystem()
	w := request(t, "GET", fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem), nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// configs returns 403 when a user tries to get config for a different user.
func Test_GetUserConfig_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID1 := makeUserID()
	userID2 := makeUserID()
	subsystem := makeSubsystem()
	w := requestAsUser(t, userID1, "GET", fmt.Sprintf("/api/configs/user/%s/%s", userID2, subsystem), nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// configs returns 404 if there's never been any configuration for that
// subsystem.
func Test_GetUserConfig_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem := makeSubsystem()
	w := requestAsUser(t, userID, "GET", fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// configs returns 401 to requests without authentication.
func Test_PostUserConfig_Anonymous(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem := makeSubsystem()
	w := request(t, "POST", fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem), nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// configs returns 403 when a user tries to set config for a different user.
func Test_PostUserConfig_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID1 := makeUserID()
	userID2 := makeUserID()
	subsystem := makeSubsystem()
	w := requestAsUser(t, userID1, "POST", fmt.Sprintf("/api/configs/user/%s/%s", userID2, subsystem), nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Posting to a configuration sets it so that you can get it again.
func Test_PostUserConfig_CreatesConfig(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem := makeSubsystem()
	content := jsonObject(makeConfig())
	endpoint := fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem)
	{
		w := requestAsUser(t, userID, "POST", endpoint, content.Reader(t))
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
	{
		w := requestAsUser(t, userID, "GET", endpoint, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content)
	}
}

// Posting to a configuration sets it so that you can get it again.
func Test_PostUserConfig_UpdatesConfig(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem := makeSubsystem()
	content1 := jsonObject(makeConfig())
	content2 := jsonObject(makeConfig())
	endpoint := fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem)
	{
		requestAsUser(t, userID, "POST", endpoint, content1.Reader(t))
		w := requestAsUser(t, userID, "POST", endpoint, content2.Reader(t))
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
	{
		w := requestAsUser(t, userID, "GET", endpoint, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content2)
	}
}

// Different subsystems can have different configurations.
func Test_PostUserConfig_MultipleSubsystems(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem1 := makeSubsystem()
	subsystem2 := makeSubsystem()
	content1 := jsonObject(makeConfig())
	content2 := jsonObject(makeConfig())
	endpoint1 := fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem1)
	endpoint2 := fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem2)
	requestAsUser(t, userID, "POST", endpoint1, content1.Reader(t))
	requestAsUser(t, userID, "POST", endpoint2, content2.Reader(t))
	{
		w := requestAsUser(t, userID, "GET", endpoint1, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content1)
	}
	{
		w := requestAsUser(t, userID, "GET", endpoint2, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content2)
	}
}

// Different users can have different configurations.
func Test_PostUserConfig_MultipleUsers(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID1 := makeUserID()
	userID2 := makeUserID()
	subsystem := makeSubsystem()
	content1 := jsonObject(makeConfig())
	content2 := jsonObject(makeConfig())
	endpoint1 := fmt.Sprintf("/api/configs/user/%s/%s", userID1, subsystem)
	endpoint2 := fmt.Sprintf("/api/configs/user/%s/%s", userID2, subsystem)
	requestAsUser(t, userID1, "POST", endpoint1, content1.Reader(t))
	requestAsUser(t, userID2, "POST", endpoint2, content2.Reader(t))
	{
		w := requestAsUser(t, userID1, "GET", endpoint1, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content1)
	}
	{
		w := requestAsUser(t, userID2, "GET", endpoint2, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content2)
	}
}

// configs returns 401 to requests without authentication.
func Test_GetOrgConfig_Anonymous(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID := makeOrgID()
	subsystem := makeSubsystem()
	w := request(t, "GET", fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem), nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func Test_GetOrgConfig_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID1 := makeOrgID()
	orgID2 := makeOrgID()
	subsystem := makeSubsystem()
	w := requestAsOrg(t, orgID1, "GET", fmt.Sprintf("/api/configs/org/%s/%s", orgID2, subsystem), nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// configs returns 404 if there's no such subsystem.
func Test_GetOrgConfig_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID := makeOrgID()
	subsystem := makeSubsystem()
	w := requestAsOrg(t, orgID, "GET", fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// configs returns 401 to requests without authentication.
func Test_PostOrgConfig_Anonymous(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID := makeOrgID()
	subsystem := makeSubsystem()
	w := request(t, "POST", fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem), nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// configs returns 403 when a user tries to set config for a different user.
func Test_PostOrgConfig_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID1 := makeOrgID()
	orgID2 := makeOrgID()
	subsystem := makeSubsystem()
	w := requestAsOrg(t, orgID1, "POST", fmt.Sprintf("/api/configs/org/%s/%s", orgID2, subsystem), nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// Posting to a configuration sets it so that you can get it again.
func Test_PostOrgConfig_CreatesConfig(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID := makeOrgID()
	subsystem := makeSubsystem()
	content := jsonObject(makeConfig())
	endpoint := fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem)
	{
		w := requestAsOrg(t, orgID, "POST", endpoint, content.Reader(t))
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
	{
		w := requestAsOrg(t, orgID, "GET", endpoint, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content)
	}
}

// Posting to a configuration sets it so that you can get it again.
func Test_PostOrgConfig_UpdatesConfig(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID := makeOrgID()
	subsystem := makeSubsystem()
	content1 := jsonObject(makeConfig())
	content2 := jsonObject(makeConfig())
	endpoint := fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem)
	{
		requestAsOrg(t, orgID, "POST", endpoint, content1.Reader(t))
		w := requestAsOrg(t, orgID, "POST", endpoint, content2.Reader(t))
		assert.Equal(t, http.StatusNoContent, w.Code)
	}
	{
		w := requestAsOrg(t, orgID, "GET", endpoint, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content2)
	}
}

// Different subsystems can have different configurations.
func Test_PostOrgConfig_MultipleSubsystems(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID := makeOrgID()
	subsystem1 := makeSubsystem()
	subsystem2 := makeSubsystem()
	content1 := jsonObject(makeConfig())
	content2 := jsonObject(makeConfig())
	endpoint1 := fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem1)
	endpoint2 := fmt.Sprintf("/api/configs/org/%s/%s", orgID, subsystem2)
	requestAsOrg(t, orgID, "POST", endpoint1, content1.Reader(t))
	requestAsOrg(t, orgID, "POST", endpoint2, content2.Reader(t))
	{
		w := requestAsOrg(t, orgID, "GET", endpoint1, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content1)
	}
	{
		w := requestAsOrg(t, orgID, "GET", endpoint2, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content2)
	}
}

// Different users can have different configurations.
func Test_PostOrgConfig_MultipleOrgs(t *testing.T) {
	setup(t)
	defer cleanup(t)

	orgID1 := makeOrgID()
	orgID2 := makeOrgID()
	subsystem := makeSubsystem()
	content1 := jsonObject(makeConfig())
	content2 := jsonObject(makeConfig())
	endpoint1 := fmt.Sprintf("/api/configs/org/%s/%s", orgID1, subsystem)
	endpoint2 := fmt.Sprintf("/api/configs/org/%s/%s", orgID2, subsystem)
	requestAsOrg(t, orgID1, "POST", endpoint1, content1.Reader(t))
	requestAsOrg(t, orgID2, "POST", endpoint2, content2.Reader(t))
	{
		w := requestAsOrg(t, orgID1, "GET", endpoint1, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content1)
	}
	{
		w := requestAsOrg(t, orgID2, "GET", endpoint2, nil)
		assert.Equal(t, parseJSON(t, w.Body.Bytes()), content2)
	}
}

func Test_GetCortexConfigs_Empty(t *testing.T) {
	setup(t)
	defer cleanup(t)

	noConfigs := api.CortexConfigsView{Configs: []configs.CortexConfig{}}
	endpoint := fmt.Sprintf("/private/api/configs/cortex?since=15m")
	w := request(t, "GET", endpoint, nil)
	var foundConfigs api.CortexConfigsView
	err := json.Unmarshal(w.Body.Bytes(), &foundConfigs)
	require.NoError(t, err, "Could not unmarshal JSON")
	assert.Equal(t, foundConfigs, noConfigs)
}
