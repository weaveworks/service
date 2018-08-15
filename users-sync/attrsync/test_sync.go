package attrsync

import (
	"github.com/weaveworks/common/signals"
)

var _ signals.SignalReceiver = &AttributeSyncer{}
