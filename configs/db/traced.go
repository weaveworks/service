package db

import (
	"github.com/Sirupsen/logrus"
)

// traced adds logrus trace lines on each db call
type traced struct {
	d DB
}

func (t traced) trace(name string, args ...interface{}) {
	logrus.Debugf("%s: %#v", name, args)
}

func (t traced) Close() (err error) {
	defer func() { t.trace("Close", err) }()
	return t.d.Close()
}
