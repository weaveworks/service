package marketing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/service/users/marketing"
)

var now = time.Now()
var yesterday = now.Add(-24 * time.Hour)

func TestMergeProspect(t *testing.T) {
	p1 := marketing.Prospect{
		Email:             "foo@bar.com",
		SignupSource:      "gcp",
		ServiceCreatedAt:  now,
		ServiceLastAccess: now,
		CampaignID:        "123",
		LeadSource:        "baz",
	}

	p2 := marketing.Prospect{
		Email:             "",
		SignupSource:      "",
		ServiceCreatedAt:  yesterday,
		ServiceLastAccess: yesterday,
		CampaignID:        "",
		LeadSource:        "",
	}

	p3 := p1.Merge(p2)
	assert.Equal(t, "foo@bar.com", p3.Email)
	assert.Equal(t, "gcp", p3.SignupSource)
	assert.Equal(t, now, p3.ServiceCreatedAt)
	assert.Equal(t, now, p3.ServiceLastAccess)
	assert.Equal(t, "123", p3.CampaignID)
	assert.Equal(t, "baz", p3.LeadSource)

	p4 := p2.Merge(p1)
	assert.Equal(t, "foo@bar.com", p4.Email)
	assert.Equal(t, "gcp", p4.SignupSource)
	assert.Equal(t, now, p4.ServiceCreatedAt)
	assert.Equal(t, now, p4.ServiceLastAccess)
	assert.Equal(t, "123", p4.CampaignID)
	assert.Equal(t, "baz", p4.LeadSource)
}
