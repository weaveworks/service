package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
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
	content := jsonObject{"arbitrary": "config"}
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
	content1 := jsonObject{"arbitrary1": "config1"}
	content2 := jsonObject{"arbitrary2": "config2"}
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
