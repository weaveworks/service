package types

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/lib/pq"
)

// SlackMessage is a Slack API payload with the message text and some options
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	LinkNames   bool              `json:"link_names,omitempty"`
	Attachments []SlackAttachment `json:"attachments"`
}

// SlackAttachment describes slack attachment
type SlackAttachment struct {
	Title     string   `json:"title,omitempty"`
	TitleLink string   `json:"title_link,omitempty"`
	Pretext   string   `json:"pretext,omitempty"`
	Fallback  string   `json:"fallback,omitempty"`
	Text      string   `json:"text"`
	Author    string   `json:"author_name,omitempty"`
	Color     string   `json:"color,omitempty"`
	MrkdwnIn  []string `json:"mrkdwn_in,omitempty"`
}

// EmailMessage contains the required fields for formatting email messages
type EmailMessage struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// BrowserMessage contains the required fields for formatting browser notifications
type BrowserMessage struct {
	Type        string            `json:"type"`
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments"`
	Timestamp   time.Time         `json:"timestamp"`
}

// StackdriverMessage contains is a stackdriver log entry.
// See https://cloud.google.com/logging/docs/view/logs_index for more about entries.
type StackdriverMessage struct {
	// Timestamp is the time of the entry. If zero, the current time is used.
	Timestamp time.Time

	// Payload is log entry payload. Its type can be either a string or an object formatted as JSON.
	Payload json.RawMessage

	// Labels optionally specifies key/value labels for the log entry.
	Labels map[string]string
}

// Event is a single instance of something for the user to be informed of
type Event struct {
	ID           string                     `json:"id"`
	Type         string                     `json:"type"`
	InstanceID   string                     `json:"instance_id"`
	InstanceName string                     `json:"instance_name"`
	Timestamp    time.Time                  `json:"timestamp"`
	Messages     map[string]json.RawMessage `json:"messages"`
	Text         *string                    `json:"text"`
	Metadata     map[string]string          `json:"metadata"`
	Attachments  []Attachment               `json:"attachments"`
}

// Attachment is a "rich" text document in a given format, ie markdown
type Attachment struct {
	Format string `json:"format,omitempty"`
	Body   string `json:"body,omitempty"`
}

// EventType is an identifier describing the type of the event.
// Example event types are ‘flux update’, ‘alert firing’, ‘probe connected’
type EventType struct {
	Name                 string   `json:"name"`
	DisplayName          string   `json:"display_name"`
	Description          string   `json:"description"`
	DefaultReceiverTypes []string `json:"default_receiver_types"`
	HideUIConfig         bool     `json:"hide_ui_config"`
	// In most cases FeatureFlag is not included, and will be blank and therefore omitted.
	FeatureFlag string `json:"feature_flag,omitempty"`
}

// Receiver is a specific configured method for a notification to be delivered
type Receiver struct {
	ID          string          `json:"id"`
	RType       string          `json:"type"`
	InstanceID  string          `json:"instance_id"`
	AddressData json.RawMessage `json:"address_data"`
	EventTypes  []string        `json:"event_types"`
}

// // ReceiverType is a kind of receiver. For example, ‘email’ or ‘slack’.
const (
	// SlackReceiver is the type of receiver for slack notifications
	SlackReceiver = "slack"
	// EmailReceiver is the type of receiver for email notifications
	EmailReceiver = "email"
	// BrowserReceiver is the type of receiver for browser notifications
	BrowserReceiver = "browser"
	// StackdriverReceiver is the type of receiver for Stackdriver
	StackdriverReceiver = "stackdriver"
)

// Notification is the actual message in data delivered to a user from address.
// One event may trigger multiple notifications if multiple receivers are configured.
type Notification struct {
	ReceiverType string          `json:"receiver_type"`
	InstanceID   string
	Address      json.RawMessage `json:"address"`
	Data         json.RawMessage `json:"data"`
	Event        Event           `json:"event"`
}

// You can't get an sql.Row from an sql.Rows, but you can scan a row from either.
type scannable interface {
	Scan(...interface{}) error
}

// EventTypeFromRow expects the row to contain (name, displayName, description, defaultReceiverTypes, featureFlag)
func EventTypeFromRow(row scannable) (EventType, error) {
	et := EventType{}
	featureFlag := sql.NullString{}
	err := row.Scan(&et.Name, &et.DisplayName, &et.Description, pq.Array(&et.DefaultReceiverTypes), &et.HideUIConfig, &featureFlag)
	if featureFlag.Valid {
		et.FeatureFlag = featureFlag.String
	}
	return et, err
}

// EventTypesFromFile loads a list of event types from file and returns them as a map {name: Event Type}
func EventTypesFromFile(path string) (map[string]EventType, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	eventTypeList := []EventType{}
	if err = json.Unmarshal(contents, &eventTypeList); err != nil {
		return nil, err
	}
	result := map[string]EventType{}
	for _, eventType := range eventTypeList {
		result[eventType.Name] = eventType
	}
	return result, nil
}

// Equals must be defined because go refuses to do equality tests for slices.
func (e EventType) Equals(other EventType) bool {
	// I tried to make this split over multiple lines but the compiler said no
	if !(e.Name == other.Name && e.DisplayName == other.DisplayName && e.Description == other.Description &&
		e.FeatureFlag == other.FeatureFlag && len(e.DefaultReceiverTypes) == len(other.DefaultReceiverTypes) &&
		e.HideUIConfig == other.HideUIConfig) {
		return false
	}
	for i := range e.DefaultReceiverTypes {
		if e.DefaultReceiverTypes[i] != other.DefaultReceiverTypes[i] {
			return false
		}
	}
	return true
}

// ReceiverFromRow expects the row to contain (id, type, instanceID, addressData, eventTypes)
func ReceiverFromRow(row scannable) (Receiver, error) {
	r := Receiver{}
	// sql driver can't convert from postgres json directly to interface{}, have to get as string and re-parse.
	addressDataBuf := []byte{}
	if err := row.Scan(&r.ID, &r.RType, &r.InstanceID, &addressDataBuf, pq.Array(&r.EventTypes)); err != nil {
		return r, err
	}
	if len(addressDataBuf) > 0 {
		if err := json.Unmarshal(addressDataBuf, &r.AddressData); err != nil {
			return r, err
		}
	}
	return r, nil
}

// EventFromRow expects the row to contain (type, instanceID, timestamp, messages)
func EventFromRow(row scannable, fields []string) (*Event, error) {
	e := Event{}
	// sql driver can't convert from postgres json directly to interface{}, have to get as string and re-parse.
	messagesBuf := []byte{}
	metadataBuf := []byte{}
	attachmentsBuff := []byte{}

	var structFields []interface{}
	for _, f := range fields {
		switch f {
		case "event_id":
			structFields = append(structFields, &e.ID)
		case "event_type":
			structFields = append(structFields, &e.Type)
		case "instance_id":
			structFields = append(structFields, &e.InstanceID)
		case "timestamp":
			structFields = append(structFields, &e.Timestamp)
		case "messages":
			structFields = append(structFields, &messagesBuf)
		case "text":
			structFields = append(structFields, &e.Text)
		case "metadata":
			structFields = append(structFields, &metadataBuf)
		default:
			return nil, fmt.Errorf("%s is an invalid field", f)
		}
	}
	structFields = append(structFields, &attachmentsBuff)

	if err := row.Scan(structFields...); err != nil {
		return nil, err
	}

	if len(messagesBuf) > 0 {
		if err := json.Unmarshal(messagesBuf, &e.Messages); err != nil {
			return nil, err
		}
	}

	if len(metadataBuf) > 0 {
		if err := json.Unmarshal(metadataBuf, &e.Metadata); err != nil {
			return nil, err
		}
	}

	if len(attachmentsBuff) > 0 {
		if err := json.Unmarshal(attachmentsBuff, &e.Attachments); err != nil {
			return nil, err
		}
	}

	return &e, nil
}
