package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Org_BillingProviderGCP(t *testing.T) {
	setup(t)
	defer cleanup(t)

	user, org := getOrg(t)
	gcp, err := database.CreateGCP(context.TODO(), "acc", "cons", "sub/1", "standard")
	assert.NoError(t, err)
	err = database.SetOrganizationGCP(context.TODO(), org.ExternalID, gcp.AccountID)
	assert.NoError(t, err)

	w := httptest.NewRecorder()
	r := requestAs(t, user, "GET", "/api/users/org/"+org.ExternalID, nil)
	app.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	body := map[string]interface{}{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "gcp", body["billingProvider"])
}
