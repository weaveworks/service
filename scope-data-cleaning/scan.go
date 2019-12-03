package main

/*

bigScan reads the entire DynamoDB table and writes out a summary text
file, where each line shows the number of records for one hour stored
by one instance

The partition key for the table is named 'hour' and formatted as
instance-hour - instance is a number mapping back to the Weave Cloud
user and hour is since the epoch, e.g. 431200 is 2019-03-11 16:00:00 UTC.

*/

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type bigScanner struct {
	segments  int
	tableName string

	dynamoDB *dynamodb.DynamoDB
}

var (
	pagesScanned = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "scope",
		Name:      "pages_scanned_total",
		Help:      "Total number of pages scanned.",
	})
)

func bigScan(config *aws.Config, tableName string, segments int) {
	var (
		scanner bigScanner
	)

	scanner.segments = segments
	scanner.tableName = tableName

	session := session.New(config)
	scanner.dynamoDB = dynamodb.New(session)

	var group sync.WaitGroup
	group.Add(scanner.segments)
	totals := newBigSummary()
	var totalsMutex sync.Mutex

	// Run multiple goroutines in parallel to increase throughput from DynamoDB
	for segment := 0; segment < scanner.segments; segment++ {
		go func(segment int) {
			// Each goroutine gets a separate data structure so we can
			// accumulate counts without locking.
			handler := newHandler()
			err := scanner.segmentScan(segment, handler)
			checkFatal(err)
			// Once the segment is finished we add those totals into the overall total.
			totalsMutex.Lock()
			totals.accumulate(handler.summary)
			totalsMutex.Unlock()
			group.Done()
		}(segment)
	}
	group.Wait()
	fmt.Printf("\n")
	totals.print()
}

func (sc bigScanner) segmentScan(segment int, handler handler) error {
	input := &dynamodb.ScanInput{
		TableName:            aws.String(sc.tableName),
		ProjectionExpression: aws.String("#h"),
		// Need to do this because "hour" is a reserved word
		ExpressionAttributeNames: map[string]*string{"#h": aws.String(hourField)},
		Segment:                  aws.Int64(int64(segment)),
		TotalSegments:            aws.Int64(int64(sc.segments)),
	}

	return sc.dynamoDB.ScanPages(input, handler.handlePage)
}

type bigSummary struct {
	counts map[string]int // map instance-hour key to number of records
}

func newBigSummary() bigSummary {
	return bigSummary{
		counts: map[string]int{},
	}
}

func (s *bigSummary) accumulate(b bigSummary) {
	for k, v := range b.counts {
		s.counts[k] += v
	}
}

func (s bigSummary) print() {
	for user, count := range s.counts {
		fmt.Printf("%s, %d\n", user, count)
	}
}

type handler struct {
	summary bigSummary
}

func newHandler() handler {
	return handler{
		summary: newBigSummary(),
	}
}

func (h *handler) reset() {
	h.summary.counts = map[string]int{}
}

// this is where the real work of bigScan happens
func (h *handler) handlePage(page *dynamodb.ScanOutput, lastPage bool) bool {
	pagesScanned.Inc()
	for _, m := range page.Items {
		v := m[hourField]
		if v.S != nil {
			key := *v.S
			h.summary.counts[key]++
		}
	}
	return true
}
