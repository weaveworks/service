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

func Test_GetUserConfig_Unauthorized(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID1 := makeUserID()
	userID2 := makeUserID()
	subsystem := makeSubsystem()
	w := requestAs(t, userID1, "GET", fmt.Sprintf("/api/configs/user/%s/%s", userID2, subsystem), nil)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// configs returns 404 if there's no such subsystem.
func Test_GetUserConfig_NotFound(t *testing.T) {
	setup(t)
	defer cleanup(t)

	userID := makeUserID()
	subsystem := makeSubsystem()
	w := requestAs(t, userID, "GET", fmt.Sprintf("/api/configs/user/%s/%s", userID, subsystem), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
