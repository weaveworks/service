package balance

import (
	"github.com/go-kit/kit/log"
	"github.com/weaveworks/common/logging"
)

type gokitAdapter struct {
	i logging.Interface
}

func (a gokitAdapter) Log(keyvals ...interface{}) error {
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, log.ErrMissingValue)
	}
	fields := logging.Fields{}
	for i := 0; i < len(keyvals); i += 2 {
		fields[keyvals[i].(string)] = keyvals[i+1]
	}
	a.i.WithFields(fields).Infoln()
	return nil
}
