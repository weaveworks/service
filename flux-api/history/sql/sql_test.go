// +build nats

package sql

import (
	"net/url"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/service/flux-api/db"
	"github.com/weaveworks/service/flux-api/history"
	"github.com/weaveworks/service/flux-api/service"
)

var (
	dbURL = "postgres://postgres@postgres:5432?sslmode=disable"
)

func bailIfErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func newSQL(t *testing.T) history.DB {
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Migrate(dbURL, "../../db/migrations/postgres"); err != nil {
		t.Fatal(err)
	}
	db, err := NewSQL(u.Scheme, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHistoryLog(t *testing.T) {
	instance := service.InstanceID("instance")
	db := newSQL(t)
	defer db.Close()

	bailIfErr(t, db.LogEvent(instance, event.Event{
		ServiceIDs: []flux.ResourceID{flux.MustParseResourceID("namespace/service")},
		Type:       "test",
		Message:    "event 1",
		EndedAt:    time.Now().UTC(),
	}))
	bailIfErr(t, db.LogEvent(instance, event.Event{
		ServiceIDs: []flux.ResourceID{flux.MustParseResourceID("namespace/other")},
		Type:       "test",
		Message:    "event 3",
		EndedAt:    time.Now().UTC(),
	}))
	bailIfErr(t, db.LogEvent(instance, event.Event{
		ServiceIDs: []flux.ResourceID{flux.MustParseResourceID("namespace/service")},
		Type:       "test",
		Message:    "event 2",
		EndedAt:    time.Now().UTC(),
	}))

	es, err := db.EventsForService(instance, flux.MustParseResourceID("namespace/service"), time.Now().UTC(), -1, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 2 {
		t.Fatalf("Expected 2 events, got %d\n", len(es))
	}
	checkInDescOrder(t, es)

	es, err = db.AllEvents(instance, time.Now().UTC(), -1, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 3 {
		t.Fatalf("Expected 3 events, got %#v\n", es)
	}
	checkInDescOrder(t, es)
}

func checkInDescOrder(t *testing.T, events []event.Event) {
	last := time.Now()
	for _, event := range events {
		if event.StartedAt.After(last) {
			t.Fatalf("Events out of order: %+v > %s", event, last)
		}
		last = event.StartedAt
	}
}
