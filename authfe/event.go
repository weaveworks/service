package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/fluent/fluent-logger-golang/fluent"
	log "github.com/sirupsen/logrus"
)

const maxBufferedEvents = 1000

// Event is a user event to be sent to out analytics system
type Event struct {
	ID             string `msg:"event"`
	SessionID      string `msg:"session_id"`
	Product        string `msg:"product"`
	Version        string `msg:"version"`
	UserAgent      string `msg:"user_agent"`
	ClientID       string `msg:"client_id"`
	OrganizationID string `msg:"org_id"`
	UserID         string `msg:"user_id"`
	IPAddress      string `msg:"ip_address"`
	Values         string `msg:"values"`
}

// TimedEvent tracks the time an event occurred
type TimedEvent struct {
	Event *Event
	Time  time.Time
}

// EventLogger logs events to the analytics system
type EventLogger struct {
	stop   chan struct{}
	events chan TimedEvent
	logger *fluent.Fluent
}

// NewEventLogger creates a new EventLogger.
func NewEventLogger(fluentHostPort string) (*EventLogger, error) {
	host, port, err := net.SplitHostPort(fluentHostPort)
	if err != nil {
		return nil, err
	}
	intPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, err
	}
	logger, err := fluent.New(fluent.Config{
		FluentPort:   intPort,
		FluentHost:   host,
		AsyncConnect: true,
		MaxRetry:     -1,
	})
	if err != nil {
		return nil, err
	}

	el := &EventLogger{
		stop:   make(chan struct{}),
		events: make(chan TimedEvent, maxBufferedEvents),
		logger: logger,
	}
	go el.logLoop()
	return el, nil
}

func (el *EventLogger) post(e TimedEvent) {
	if err := el.logger.PostWithTime("events", e.Time, e.Event); err != nil {
		log.Warnf("EventLogger: failed to log event: %v", e)
	}
}

func (el *EventLogger) logLoop() {
	for done := false; !done; {
		select {
		case event := <-el.events:
			el.post(event)
		case <-el.stop:
			done = true
		}
	}

	// flush remaining events
	for done := false; !done; {
		select {
		case event := <-el.events:
			el.post(event)
		default:
			done = true
		}
	}

	el.logger.Close()
}

// Close closes and deallocates the event logger
func (el *EventLogger) Close() error {
	close(el.stop)
	return nil
}

// LogEvent logs an event to the analytics system
func (el *EventLogger) LogEvent(ev Event) error {
	select {
	case <-el.stop:
		return fmt.Errorf("Stopping, discarding event: %v", ev)
	default:
	}
	e := TimedEvent{
		Event: &ev,
		Time:  time.Now(),
	}
	select {
	case el.events <- e: // Put event in the channel unless it is full
		return nil
	default:
		// full
	}
	eventsDiscardedCount.Inc()
	return fmt.Errorf("Reached event buffer limit (%d), discarding event: %v", maxBufferedEvents, ev)
}

// HTTPEventExtractor extracts an event from an http requests indicating whether it should be loggged
type HTTPEventExtractor func(*http.Request) (Event, bool)

// HTTPEventLogger logs an events extracted from an http request
type HTTPEventLogger struct {
	Extractor HTTPEventExtractor
	Logger    *EventLogger
}

// Wrap implements middleware.Wrap()
func (el HTTPEventLogger) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if event, shouldLog := el.Extractor(r); shouldLog {
			if err := el.Logger.LogEvent(event); err != nil {
				log.Warnf("HTTPEventLogger: failed to log event: %v", err)
			}
		}
		next.ServeHTTP(w, r)
	})
}
