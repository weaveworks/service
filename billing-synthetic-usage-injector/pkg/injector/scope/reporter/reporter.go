package reporter

import (
	"strconv"

	"github.com/weaveworks/scope/probe"
	"github.com/weaveworks/scope/report"
)

// NewFakeScopeReporter instantiates a new probe.Reporter which reports fake data to Scope.
func NewFakeScopeReporter(numNodes uint) probe.Reporter {
	return probe.ReporterFunc("fake-node-"+strconv.Itoa(int(numNodes)), func() (report.Report, error) {
		r := report.MakeReport()
		for i := uint(0); i < numNodes; i++ {
			id := strconv.Itoa(int(i))
			r.Host.AddNode(report.MakeNode(id))
		}
		return r, nil
	})
}
