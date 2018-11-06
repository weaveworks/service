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
		FirstName:         "Johnny Reed",
		LastName:          "McKinzie",
		Company:           "TDE",
	}

	p2 := marketing.Prospect{
		Email:             "",
		SignupSource:      "",
		ServiceCreatedAt:  yesterday,
		ServiceLastAccess: yesterday,
		CampaignID:        "",
		LeadSource:        "",
	}

	m12 := p1.Merge(p2)
	assert.Equal(t, p1, m12)

	m21 := p2.Merge(p1)
	assert.Equal(t, p1, m21)
}
