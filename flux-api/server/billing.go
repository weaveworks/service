package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	billing "github.com/weaveworks/billing-client"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/flux/event"
)

// BillingClient covers our use of billing.Client
type BillingClient interface {
	AddAmounts(uniqueKey, internalInstanceID string, timestamp time.Time, amounts billing.Amounts, metadata map[string]string) error
}

// NoopBillingClient is a BillingClient which does nothing
type NoopBillingClient struct{}

// AddAmounts pretends to add amounts
func (NoopBillingClient) AddAmounts(uniqueKey, internalInstanceID string, timestamp time.Time, amounts billing.Amounts, metadata map[string]string) error {
	return nil
}

func (s *Server) emitBillingRecord(ctx context.Context, event event.Event) error {
	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return err
	}
	// convert _ to - to match other billing amount types
	var amountType = billing.AmountType(fmt.Sprintf("flux-%s", strings.Replace(event.Type, "_", "-", -1)))
	amounts := billing.Amounts{
		// TODO If multiple services are updated, should we still count 1?
		amountType: 1,
	}
	now := time.Now().UTC()
	return s.billingClient.AddAmounts(
		strconv.FormatInt(int64(event.ID), 10),
		userID,
		now,
		amounts,
		nil,
	)
}
