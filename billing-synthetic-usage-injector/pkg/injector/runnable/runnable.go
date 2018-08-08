package runnable

import "github.com/weaveworks/common/signals"

// Runnable is the abstraction around components reporting usage, which
// typically have to be started and stopped.
type Runnable interface {
	signals.SignalReceiver // i.e. Stop() error
	Start()
}
