package grpc

import "github.com/weaveworks/service/common/billing/provider"

// This file amends the generated struct by protobuf in billing.pb.go

// BilledExternally returns true if this billing account is set
// to external.
func (b *BillingAccount) BilledExternally() bool {
	if b == nil {
		return false
	}
	return b.Provider == provider.External
}
