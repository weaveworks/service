package configmanager

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/badoux/checkmail"
	"github.com/gorilla/mux"
	"github.com/lib/pq" // Import the postgres sql driver
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	uuid "github.com/satori/go.uuid"

	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/common/users"
	"github.com/weaveworks/service/notification-configmanager/types"
	"github.com/weaveworks/service/notification-configmanager/utils"
	"github.com/weaveworks/service/notification-eventmanager"
	serviceUsers "github.com/weaveworks/service/users"
	usersClient "github.com/weaveworks/service/users/client"
	_ "gopkg.in/mattes/migrate.v1/driver/postgres" // Import the postgres migrations driver
	"gopkg.in/mattes/migrate.v1/migrate"
)

const (
	// MaxEventsList is the highest number of events that can be requested in one list call
	MaxEventsList = 100
	// timeout waiting for database connection to be established
	dbConnectTimeout = 5 * time.Minute
)

var (
	// isWebHookPath regexp checks string contains only letters, numbers and slashes
	// for url.Path in slack webhook (services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX)
	isWebHookPath           = regexp.MustCompile(`^[A-Za-z0-9\/]+$`).MatchString
	databaseRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "notification",
		Name:      "database_request_duration_seconds",
		Help:      "Time spent (in seconds) doing database requests.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "status_code"})

	eventsToEventmanagerTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_to_eventmanager_total",
		Help: "Number of events sent to eventmanager.",
	}, []string{"event_type"})
	eventsToEventmanagerError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "event_to_eventmanager_errors_total",
		Help: "Number of errors sending event to eventmanager.",
	}, []string{"event_type"})
)

func init() {
	prometheus.MustRegister(
		databaseRequestDuration,
		eventsToEventmanagerTotal,
		eventsToEventmanagerError,
	)
	rand.Seed(time.Now().UnixNano()) // set seed for retry function
}

// https://upgear.io/blog/simple-golang-retry-function/
func retry(attempts int, sleep time.Duration, f func() error) error {
	if err := f(); err != nil {
		if s, ok := err.(stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			// Add some randomness to prevent creating a Thundering Herd
			jitter := time.Duration(rand.Int63n(int64(sleep)))
			sleep = sleep + jitter/2

			time.Sleep(sleep)
			return retry(attempts, 2*sleep, f) // exponential backoff
		}
		return err
	}

	return nil
}

type stop struct {
	error
}

// Config is the configuration for Config Manager, not to be confused with the user configs it manages!
type Config struct {
	databaseURI     string
	migrationsDir   string
	eventTypesPath  string
	eventManagerURL string
	usersServiceURL string
}

// RegisterFlags registers CLI flags to configure a Config Manager
func (c *Config) RegisterFlags() {
	flag.StringVar(&c.databaseURI, "database.uri", "", "URI where the database can be found")
	flag.StringVar(&c.migrationsDir, "database.migrations", "", "Path where the database migration files can be found")
	flag.StringVar(&c.eventTypesPath, "eventtypes", "", "Path to a JSON file defining available event types")
	flag.StringVar(&c.eventManagerURL, "eventManagerURL", "", "URL to connect to event manager")
	flag.StringVar(&c.usersServiceURL, "usersServiceURL", "users.default:4772", "URL to connect to users service")
}

// ConfigManager is the struct we hang all methods off
type ConfigManager struct {
	db          *utils.DB
	eventURL    string
	usersClient serviceUsers.UsersClient
}

func waitForDBConnection(db *sql.DB) error {
	deadline := time.Now().Add(dbConnectTimeout)
	for tries := 0; time.Now().Before(deadline); tries++ {
		err := db.Ping()
		if err == nil {
			return nil
		}
		log.Warnf("db connection not established, error: %s; retrying...", err)
		time.Sleep(time.Second << uint(tries))
	}
	return fmt.Errorf("db connection not established after %s", dbConnectTimeout)
}

// New creates a new ConfigManager from a Config
func New(config Config) (*ConfigManager, error) {
	if config.databaseURI == "" {
		return nil, errors.New("Database URI is required")
	}

	db, err := sql.Open("postgres", config.databaseURI)
	if err != nil {
		return nil, err
	}

	if err := waitForDBConnection(db); err != nil {
		return nil, errors.Wrap(err, "cannot establish db connection")
	}

	wrappedDB := utils.NewDB(db, databaseRequestDuration)

	if config.migrationsDir != "" {
		log.Infof("Running Database Migrations...")
		if errs, ok := migrate.UpSync(config.databaseURI, config.migrationsDir); !ok {
			for _, err := range errs {
				log.Error(err)
			}
			return nil, errors.New("Database migrations failed")
		}
	}

	var uclient *users.Client
	if config.usersServiceURL == "mock" {
		uclient = &users.Client{UsersClient: usersClient.MockClient{}}
	} else {
		uclient, err = users.NewClient(users.Config{HostPort: config.usersServiceURL})
		if err != nil {
			return nil, errors.Wrapf(err, "cannot create users client: %v", config.usersServiceURL)
		}
	}

	c := &ConfigManager{
		db:          wrappedDB,
		eventURL:    config.eventManagerURL,
		usersClient: uclient,
	}

	if config.eventTypesPath != "" {
		eventTypes, err := types.EventTypesFromFile(config.eventTypesPath)
		if err != nil {
			return nil, err
		}
		log.Infof("Synchronizing %d event types with DB", len(eventTypes))
		err = c.syncEventTypes(eventTypes)
		if err != nil {
			return nil, err
		}
		log.Infof("Synchronized event types")
	}

	if config.eventManagerURL == "" {
		return nil, errors.New("eventmanager URL is required")
	}

	return c, nil
}

// Register HTTP handlers
func (c *ConfigManager) Register(r *mux.Router) {
	for _, route := range []struct {
		name, method, path string
		handler            http.Handler
	}{
		{"list_event_types", "GET", "/api/notification/config/eventtypes", withNoArgs(c.httpListEventTypes)},

		{"list_receivers", "GET", "/api/notification/config/receivers", withInstance(c.listReceivers)},
		{"create_receiver", "POST", "/api/notification/config/receivers", withInstance(c.httpCreateReceiver)},
		{"get_receiver", "GET", "/api/notification/config/receivers/{id}", withInstanceAndID(c.getReceiver)},
		{"update_receiver", "PUT", "/api/notification/config/receivers/{id}", withInstanceAndID(c.updateReceiver)},
		{"delete_receiver", "DELETE", "/api/notification/config/receivers/{id}", withInstanceAndID(c.deleteReceiver)},

		{"list_receivers_for_event", "GET", "/api/notification/config/receivers_for_event/{id}", withInstanceAndID(c.getReceiversForEvent)},

		{"create_event", "POST", "/api/notification/events", withInstance(c.createEvent)},
		{"get_events", "GET", "/api/notification/events", withInstance(c.getEvents)},

		{"healthcheck", "GET", "/api/notification/config/healthcheck", withNoArgs(c.healthCheck)},
	} {
		r.Handle(route.path, route.handler).Methods(route.method).Name(route.name)
	}
}

// healthCheck handles a very simple health check
func (c *ConfigManager) healthCheck(r *http.Request) (interface{}, int, error) {
	return nil, http.StatusOK, nil
}

// for each row returned from query, calls callback
func (c *ConfigManager) forEachRow(rows *sql.Rows, callback func(*sql.Rows) error) error {
	var err error
	for err == nil && rows.Next() {
		err = callback(rows)
	}
	if err == nil {
		err = rows.Err()
	}
	return err
}

// Executes the given function in a transaction, and rolls back or commits depending on if the function errors.
// Ignores errors from rollback.
func (c *ConfigManager) withTx(method string, f func(tx *utils.Tx) error) error {
	tx, err := c.db.Begin(method)
	if err != nil {
		return err
	}
	err = f(tx)
	if err == nil {
		return tx.Commit()
	}
	_ = tx.Rollback()

	if strings.Contains(err.Error(), "pq: duplicate key value violates unique constraint \"instances_initialized_pkey\"") {
		// if error is pq: duplicate key value violates unique constraint "instances_initialized_pkey"
		// instance already initialized, ignore this error
		return nil
	}

	return err
}

// Called before any handlers involving receivers, to initialize receiver defaults for the instance
// if it hasn't been already.
func (c *ConfigManager) checkInstanceDefaults(instanceID string) error {
	return c.withTx("check_instance_defaults_tx", func(tx *utils.Tx) error {
		// Test if instance is already initialized
		row := tx.QueryRow("check_instance_initialized", "SELECT 1 FROM instances_initialized WHERE instance_id = $1", instanceID)
		var unused int // We're only interested if a row is present, if it is the value will be 1.
		if err := row.Scan(&unused); err == nil {
			// No err means row was present, ie. instance already initialized. We're done.
			return nil
		} else if err != sql.ErrNoRows {
			// Actual DB error
			return err
		}
		// otherwise, err == ErrNoRows, ie. instance not initialized yet.

		// Hard-coded instance defaults, at least for now.
		receiver := types.Receiver{
			RType:       types.BrowserReceiver,
			AddressData: json.RawMessage("null"),
		}
		if _, _, err := c.createReceiver(tx, receiver, instanceID); err != nil {
			return err
		}

		// finally, insert instance_id into instances_initialized to mark it as done
		_, err := tx.Exec("set_instance_initialized", "INSERT INTO instances_initialized (instance_id) VALUES ($1)", instanceID)
		return err
	})
}

func (c *ConfigManager) httpListEventTypes(r *http.Request) (interface{}, int, error) {
	result, err := c.listEventTypes(nil, getFeatureFlags(r))
	if err != nil {
		return nil, 0, err
	}
	return result, http.StatusOK, nil
}

// Return a list of event types from the DB, either using given transaction or without a transaction if nil.
// Also filter by enabled feature flags if featureFlags is not nil.
func (c *ConfigManager) listEventTypes(tx *utils.Tx, featureFlags []string) ([]types.EventType, error) {
	queryFn := c.db.Query
	if tx != nil {
		queryFn = tx.Query
	}
	// Exclude feature-flagged rows only if a) feature flags given is not nil and b) row has a feature flag
	// and c) feature flag isn't in given list of feature flags.
	rows, err := queryFn(
		"list_event_types",
		`SELECT name, display_name, description, default_receiver_types, feature_flag FROM event_types
		WHERE $1::text[] IS NULL OR feature_flag IS NULL OR feature_flag = ANY ($1::text[])`,
		pq.Array(featureFlags),
	)
	if err != nil {
		return nil, err
	}
	eventTypes := []types.EventType{}
	err = c.forEachRow(rows, func(row *sql.Rows) error {
		et, err := types.EventTypeFromRow(row)
		if err != nil {
			return err
		}
		eventTypes = append(eventTypes, et)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return eventTypes, nil
}

func (c *ConfigManager) listReceivers(r *http.Request, instanceID string) (interface{}, int, error) {
	if err := c.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	// Note we exclude event types with non-matching feature flags.
	rows, err := c.db.Query(
		"list_receivers",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r
		LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		LEFT JOIN event_types et ON (rt.event_type = et.name)
		WHERE r.instance_id = $1
		GROUP BY r.receiver_id`,
		instanceID,
	)
	if err != nil {
		return nil, 0, err
	}
	receivers := []types.Receiver{}
	err = c.forEachRow(rows, func(row *sql.Rows) error {
		r, err := types.ReceiverFromRow(row)
		if err != nil {
			return err
		}
		receivers = append(receivers, r)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return receivers, http.StatusOK, err
}

func (c *ConfigManager) httpCreateReceiver(r *http.Request, instanceID string) (interface{}, int, error) {
	if err := c.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	receiver := types.Receiver{}
	err := parseBody(r, &receiver)
	if err != nil {
		log.Errorf("cannot parse body, error: %s", err)
		return "Bad request body", http.StatusBadRequest, nil
	}
	if err := isValidAddress(receiver.AddressData, receiver.RType); err != nil {
		log.Errorf("address validation failed for %s address, error: %s", receiver.RType, err)
		return "address validation failed", http.StatusBadRequest, nil
	}
	var result interface{}
	var code int
	err = c.withTx("create_receiver_tx", func(tx *utils.Tx) error {
		var err error
		result, code, err = c.createReceiver(tx, receiver, instanceID)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return result, code, nil
}

func isValidAddress(addressData json.RawMessage, rtype string) error {
	switch rtype {
	case types.SlackReceiver:
		var addrStr string
		if err := json.Unmarshal(addressData, &addrStr); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s receiver address data %s", rtype, addressData)
		}

		url, err := url.ParseRequestURI(addrStr)
		if err != nil {
			return errors.Wrapf(err, "cannot parse URI %s", addrStr)
		}
		if url.Scheme != "https" || url.Port() != "" || url.Host != "hooks.slack.com" || !isWebHookPath(url.Path) {
			return errors.Errorf("invalid slack webhook URL %s", addrStr)
		}

	case types.EmailReceiver:
		var addrStr string
		if err := json.Unmarshal(addressData, &addrStr); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s receiver address data %s", rtype, addressData)
		}

		if err := checkmail.ValidateFormat(addrStr); err != nil {
			return errors.Wrapf(err, "invalid email address %s", addrStr)
		}

	case types.BrowserReceiver:
		return nil

	case types.StackdriverReceiver:
		fields := []string{
			"type",
			"project_id",
			"private_key_id",
			"private_key",
			"client_email",
			"client_id",
			"auth_uri",
			"token_uri",
			"auth_provider_x509_cert_url",
			"client_x509_cert_url",
		}
		var creds map[string]string
		if err := json.Unmarshal(addressData, &creds); err != nil {
			return errors.Wrapf(err, "cannot unmarshal %s receiver address data", rtype)
		}

		for _, v := range fields {
			if creds[v] == "" || (v == "type" && creds[v] != "service_account") {
				return errors.Errorf("invalid stackdriver receiver")
			}
		}
	}

	return nil
}

func (c *ConfigManager) createReceiver(tx *utils.Tx, receiver types.Receiver, instanceID string) (interface{}, int, error) {
	// Validate they only set the fields we're going to use
	if receiver.ID != "" || receiver.InstanceID != "" || len(receiver.EventTypes) != 0 {
		return "ID, instance and event types should not be specified", http.StatusBadRequest, nil
	}
	// Re-encode the address data because the sql driver doesn't understand json columns
	// TODO validate this field against the specific receiver type
	encodedAddress, err := json.Marshal(receiver.AddressData)
	if err != nil {
		return nil, 0, err // This is a server error, because this round-trip *should* work.
	}
	// In a single transaction to prevent races (receiver gets edited before we're finished setting default event types),
	// we create the new Receiver row, look up what event types it should default to handling, and add those.
	row := tx.QueryRow(
		"create_receiver",
		`INSERT INTO receivers (instance_id, receiver_type, address_data)
		VALUES ($1, $2, $3)
		RETURNING receiver_id`,
		instanceID,
		receiver.RType,
		encodedAddress,
	)
	var receiverID string
	err = row.Scan(&receiverID)
	if err != nil {
		return nil, 0, err
	}
	_, err = tx.Exec(
		"set_receiver_defaults",
		`INSERT INTO receiver_event_types (receiver_id, event_type)
			SELECT $1, name
			FROM event_types
			WHERE $2 = ANY (default_receiver_types)`,
		receiverID,
		receiver.RType,
	)
	if err != nil {
		return nil, 0, err
	}
	return receiverID, http.StatusCreated, nil
}

func (c *ConfigManager) getReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if err := c.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	row := c.db.QueryRow(
		"get_receiver",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r
		LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		LEFT JOIN event_types et ON (rt.event_type = et.name)
		WHERE r.receiver_id = $1 AND r.instance_id = $2 AND (et.feature_flag IS NULL OR et.feature_flag = ANY ($3))
		GROUP BY r.receiver_id`,
		receiverID,
		instanceID,
		pq.Array(getFeatureFlags(r)),
	)
	receiver, err := types.ReceiverFromRow(row)
	if err == sql.ErrNoRows {
		return nil, http.StatusNotFound, nil
	}
	if err != nil {
		return nil, 0, err
	}
	return receiver, http.StatusOK, nil
}

func (c *ConfigManager) createConfigChangedEvent(ctx context.Context, instanceID string, oldReceiver, receiver types.Receiver, eventTime time.Time) error {
	log.Debug("update_config Event Firing...")

	eventType := "config_changed"
	event := types.Event{
		Type:       eventType,
		InstanceID: instanceID,
		Timestamp:  eventTime,
	}

	instanceData, err := c.usersClient.GetOrganization(ctx, &serviceUsers.GetOrganizationRequest{
		ID: &serviceUsers.GetOrganizationRequest_InternalID{InternalID: instanceID},
	})
	if err != nil {
		if eventmanager.IsStatusErrorCode(err, http.StatusNotFound) {
			log.Warnf("instance name for ID %s not found for event type %s", instanceID, eventType)
			return errors.Wrap(err, "instance not found")
		}
		log.Errorf("error requesting instance data from users service for event type %s: %s", eventType, err)
		return errors.Wrapf(err, "error requesting instance data from users service for event type %s", eventType)
	}
	instanceName := instanceData.Organization.Name

	sort.Strings(oldReceiver.EventTypes)
	sort.Strings(receiver.EventTypes)

	// currently only 3 events can happen on config/notifications page
	// 1) the address changed
	// 2) the eventTypes changed
	// 3) event fired unnecessarily, and niether address or eventTypes changed. (This happens because all receivers fire PUT change events on save)
	// Currently these are distinct non-overlapping events
	if !bytes.Equal(oldReceiver.AddressData, receiver.AddressData) {
		// address changed event
		msg := fmt.Sprintf("The address for <b>%s</b> was updated!", receiver.RType)

		emailMsg, err := eventmanager.GetEmailMessage(msg, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := eventmanager.GetBrowserMessage(msg, nil, eventType)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := eventmanager.GetStackdriverMessage(msgJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message")
		}

		event.Messages = map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       json.RawMessage(fmt.Sprintf(`{"text": "*Instance:* %v\nThe address for *%s* was updated!"}`, instanceName, receiver.RType)),
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else if !reflect.DeepEqual(oldReceiver.EventTypes, receiver.EventTypes) {
		// eventTypes changed event
		// PossibleTodo: find set difference between oldEventTypes and newEventTypes -> "removed [...] and added [...]"
		msg := fmt.Sprintf("The event types for <b>%s</b> were updated from <i>%s</i> to <i>%s</i>", receiver.RType, oldReceiver.EventTypes, receiver.EventTypes)

		emailMsg, err := eventmanager.GetEmailMessage(msg, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		browserMsg, err := eventmanager.GetBrowserMessage(msg, nil, eventType)
		if err != nil {
			return errors.Wrap(err, "cannot get email message")
		}

		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return errors.Wrap(err, "cannot marshal message")
		}
		stackdriverMsg, err := eventmanager.GetStackdriverMessage(msgJSON, eventType, instanceName)
		if err != nil {
			return errors.Wrap(err, "cannot get stackdriver message for event types changed")
		}

		event.Messages = map[string]json.RawMessage{
			types.EmailReceiver:       emailMsg,
			types.BrowserReceiver:     browserMsg,
			types.SlackReceiver:       json.RawMessage(fmt.Sprintf(`{"text": "*Instance:* %v\nThe event types for *%s* were updated from _%s_ to _%s_"}`, instanceName, receiver.RType, oldReceiver.EventTypes, receiver.EventTypes)),
			types.StackdriverReceiver: stackdriverMsg,
		}
	} else {
		// nothing changed, don't send event
		return nil
	}

	rawJSONEvent, jsonErr := json.Marshal(event)
	if jsonErr != nil {
		return jsonErr
	}

	eventsToEventmanagerTotal.With(prometheus.Labels{"event_type": event.Type}).Inc()
	postEventURL := fmt.Sprintf("%s/api/notification/events", c.eventURL)
	req, err := http.NewRequest("POST", postEventURL, bytes.NewBuffer(rawJSONEvent))
	if err != nil {
		return errors.Wrap(err, "cannot create request to send config_change event")
	}

	req.Header.Set("Content-Type", "application/json")

	// try 5 times to POST our request
	retryErr := retry(5, time.Second, func() error {
		ctxWithID := user.InjectOrgID(context.Background(), instanceID)
		err = user.InjectOrgIDIntoHTTPRequest(ctxWithID, req)
		if err != nil {
			return errors.Wrap(err, "cannot inject instanceID into request")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Debugf("POSTing to events connection not established, error: %s; retrying...", err)
			return err
		}
		defer resp.Body.Close()

		s := resp.StatusCode
		switch {
		case s >= 500:
			// Retry
			log.Debugf("POSTing to events connection not established, status: %s; retrying...", s)
			return errors.Errorf("internal server error, status %d", s)
		case s >= 400:
			// Don't retry, it was client's fault
			return stop{errors.Errorf("client error, status %d", s)}
		default:
			// Happy
			return nil
		}
	})

	if retryErr != nil {
		eventsToEventmanagerError.With(prometheus.Labels{"event_type": event.Type}).Inc()
		return errors.Wrapf(err, "could not send %s event", event.Type)
	}

	return nil
}

func (c *ConfigManager) updateReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	eventTime := time.Now()
	if err := c.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	receiver := types.Receiver{}

	err := parseBody(r, &receiver)
	if err != nil {
		return "Bad request body", http.StatusBadRequest, nil
	}
	if (receiver.ID != "" && receiver.ID != receiverID) || (receiver.ID != "" && receiver.InstanceID != instanceID) {
		return "Receiver ID and instance ID cannot be modified", http.StatusBadRequest, nil
	}

	if err := isValidAddress(receiver.AddressData, receiver.RType); err != nil {
		log.Errorf("address validation failed for %s address, error: %s", receiver.RType, err)
		return "address validation failed", http.StatusBadRequest, nil
	}

	// Re-encode the address data because the sql driver doesn't understand json columns
	// TODO validate this field against the specific receiver type
	encodedAddress, err := json.Marshal(receiver.AddressData)
	if err != nil {
		return nil, 0, err // This is a server error, because this round-trip *should* work.
	}
	// We need to read some DB values to validate the new data (we could just rely on the DB's constraints, but it's hard
	// to return a meaningful error message if we do that). We do this in a transaction to prevent races.
	// We set these vars as a way of returning more values from inside the closure, since the function can only return error.
	code := http.StatusOK

	if receiver.ID == "" {
		receiver.ID = receiverID
	}

	// before transaction changes the addressData and eventTypes, get oldReceiver which has oldAddressData and oldEventTypes
	receiverOrNil, status, err := c.getReceiver(r, instanceID, receiverID)
	gotOldReceiverValues := true
	if err != nil {
		gotOldReceiverValues = false
		log.Errorf("error with c.getReceiver call. Status: %v\nError: %s", status, err)
	}

	oldReceiver, ok := receiverOrNil.(types.Receiver)
	if !ok {
		gotOldReceiverValues = false
		log.Errorf("error converting result of getReceiver call, %v to types.Receiver", receiverOrNil)
	}

	// Transaction to actually update the receiver in DB
	userErrorMsg := ""
	err = c.withTx("update_receiver_tx", func(tx *utils.Tx) error {
		// Verify receiver exists, has correct instance id and type.
		row := tx.QueryRow("check_receiver_exists", `SELECT receiver_type FROM receivers WHERE receiver_id = $1 AND instance_id = $2`, receiverID, instanceID)
		var rtype string
		err := row.Scan(&rtype)
		if err == sql.ErrNoRows {
			code = http.StatusNotFound
			return nil
		}
		if err != nil {
			return err
		}
		if rtype != receiver.RType {
			userErrorMsg = "Receiver type cannot be modified"
			code = http.StatusBadRequest
			return nil
		}
		// Verify event types list is valid by querying for items in the input but not in event_types
		rows, err := tx.Query(
			"check_new_receiver_event_types",
			`SELECT unnest FROM unnest($1::text[])
			WHERE unnest NOT IN (SELECT name FROM event_types)`,
			pq.Array(receiver.EventTypes),
		)
		if err != nil {
			return err
		}
		badTypes := []string{}
		for rows.Next() {
			var badType string
			err = rows.Scan(&badType)
			if err != nil {
				return err
			}
			badTypes = append(badTypes, badType)
		}
		err = rows.Err()
		if err != nil {
			return err
		}
		if len(badTypes) != 0 {
			userErrorMsg = fmt.Sprintf("Given event types do not exist: %s", strings.Join(badTypes, ", "))
			code = http.StatusNotFound
			return nil
		}
		// Verified all good, do the update.
		// First, update the actual record
		_, err = tx.Exec(
			"update_receiver",
			`UPDATE receivers SET (address_data) = ($2)
			WHERE receiver_id = $1`,
			receiverID,
			encodedAddress,
		)
		if err != nil {
			return err
		}
		// Delete any newly-dropped event types. Note we keep feature-flag-hidden event types,
		// since the client wouldn't have known about these so omitting them was not an intentional delete.
		_, err = tx.Exec(
			"remove_receiver_event_types",
			`DELETE FROM receiver_event_types
			WHERE receiver_id = $1 AND NOT event_type IN (
				SELECT name
				FROM event_types
				WHERE name = ANY ($2) OR NOT (feature_flag IS NULL OR feature_flag = ANY ($3))
			)`,
			receiverID,
			pq.Array(receiver.EventTypes),
			pq.Array(getFeatureFlags(r)),
		)
		if err != nil {
			return err
		}
		// Add any new event types
		_, err = tx.Exec(
			"add_receiver_event_types",
			`INSERT INTO receiver_event_types (receiver_id, event_type) (
				SELECT $1, unnest FROM unnest($2::text[])
			) ON CONFLICT DO NOTHING`,
			receiverID,
			pq.Array(receiver.EventTypes),
		)
		return err
	})

	if err != nil {
		return nil, 0, err
	}
	if code == http.StatusOK {
		// all good!
		// Fire event every time config is successfully changed
		if gotOldReceiverValues {
			go func() {
				eventErr := c.createConfigChangedEvent(context.Background(), instanceID, oldReceiver, receiver, eventTime)
				if eventErr != nil {
					log.Error(eventErr)
				}
			}()
		} else {
			log.Error("Event config_changed not sent. Old receiver values not acquired.")
		}
		return nil, code, nil
	}
	return userErrorMsg, code, nil
}

func (c *ConfigManager) deleteReceiver(r *http.Request, instanceID string, receiverID string) (interface{}, int, error) {
	if err := c.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	if _, err := uuid.FromString(receiverID); err != nil {
		// Bad identifier
		return nil, http.StatusNotFound, nil
	}
	result, err := c.db.Exec(
		"delete_receiver",
		`DELETE FROM receivers
		WHERE receiver_id = $1 AND instance_id = $2`,
		receiverID,
		instanceID,
	)
	if err != nil {
		return nil, 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, 0, err
	}
	if affected == 0 {
		return nil, http.StatusNotFound, nil
	}
	return nil, http.StatusOK, nil
}

func (c *ConfigManager) getReceiversForEvent(r *http.Request, instanceID string, eventType string) (interface{}, int, error) {
	if err := c.checkInstanceDefaults(instanceID); err != nil {
		return nil, 0, err
	}
	receivers := []types.Receiver{}
	// In the below query, note the array_remove to transform [null] to [] if there are no matching rows.
	rows, err := c.db.Query(
		"get_receivers_for_event",
		`SELECT r.receiver_id, r.receiver_type, r.instance_id, r.address_data,
			array_remove(array_agg(rt.event_type), NULL)
		FROM receivers r LEFT JOIN receiver_event_types rt ON (r.receiver_id = rt.receiver_id)
		WHERE instance_id = $1 AND event_type = $2
		GROUP BY r.receiver_id`,
		instanceID,
		eventType,
	)
	if err != nil {
		return nil, 0, err
	}
	err = c.forEachRow(rows, func(row *sql.Rows) error {
		r, err := types.ReceiverFromRow(row)
		if err != nil {
			return err
		}
		receivers = append(receivers, r)
		return nil
	})
	return receivers, http.StatusOK, err
}

func (c *ConfigManager) createEvent(r *http.Request, instanceID string) (interface{}, int, error) {

	event := types.Event{}
	if err := parseBody(r, &event); err != nil {
		return "Bad request body", http.StatusBadRequest, nil
	}
	if instanceID != event.InstanceID {
		return "Mismatching instance ids", http.StatusBadRequest, nil
	}
	// Re-encode the message data because the sql driver doesn't understand json columns
	encodedMessages, err := json.Marshal(event.Messages)
	if err != nil {
		return nil, 0, err // This is a server error, because this round-trip *should* work.
	}
	code := http.StatusOK
	userErrorMsg := ""
	err = c.withTx("add_event_tx", func(tx *utils.Tx) error {
		row := tx.QueryRow(
			"check_event_type_exists",
			`SELECT 1 FROM event_types WHERE name = $1`,
			event.Type,
		)
		var junk int
		err := row.Scan(&junk)
		if err == sql.ErrNoRows {
			userErrorMsg = "Event type does not exist"
			code = http.StatusBadRequest
			return nil
		} else if err != nil {
			return err
		}
		_, err = tx.Exec(
			"add_event",
			`INSERT INTO events (event_type, instance_id, timestamp, messages) VALUES ($1, $2, $3, $4)`,
			event.Type,
			event.InstanceID,
			event.Timestamp,
			encodedMessages,
		)
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	return userErrorMsg, code, nil
}

func (c *ConfigManager) getEvents(r *http.Request, instanceID string) (interface{}, int, error) {
	params := r.URL.Query()
	length := 50
	offset := 0
	if params.Get("length") != "" {
		l, err := strconv.Atoi(params.Get("length"))
		if err != nil {
			return "Bad length value: Not an integer", http.StatusBadRequest, nil
		}
		if l < 0 || l > MaxEventsList {
			return fmt.Sprintf("Bad length value: Must be between 0 and %d inclusive", MaxEventsList), http.StatusBadRequest, nil
		}
		length = l
	}
	if params.Get("offset") != "" {
		o, err := strconv.Atoi(params.Get("offset"))
		if err != nil {
			return "Bad offset value: Not an integer", http.StatusBadRequest, nil
		}
		if o < 0 {
			return "Bad offset value: Must be non-negative", http.StatusBadRequest, nil
		}
		offset = o
	}
	rows, err := c.db.Query(
		"get_events",
		`SELECT event_id, event_type, instance_id, timestamp, messages
		FROM events
		WHERE instance_id = $1
		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3`,
		instanceID,
		length,
		offset,
	)
	if err != nil {
		return nil, 0, err
	}
	events := []types.Event{}
	err = c.forEachRow(rows, func(row *sql.Rows) error {
		e, err := types.EventFromRow(row)
		if err != nil {
			return err
		}
		events = append(events, e)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return events, http.StatusOK, err
}

func (c *ConfigManager) syncEventTypes(eventTypes map[string]types.EventType) error {
	return c.withTx("sync_event_types_tx", func(tx *utils.Tx) error {
		oldEventTypes, err := c.listEventTypes(tx, nil)
		if err != nil {
			return err
		}
		for _, oldEventType := range oldEventTypes {
			if eventType, ok := eventTypes[oldEventType.Name]; ok {
				// We delete the entries as we see them so we know at the end what ones are completely new
				delete(eventTypes, eventType.Name)
				if !eventType.Equals(oldEventType) {
					log.Infof("Updating event type %s", eventType.Name)
					err = c.updateEventType(tx, eventType)
					if err != nil {
						return err
					}
				}
			} else {
				log.Warnf("Refusing to delete old event type %s for safety, if you really meant to do this then do so manually", oldEventType.Name)
			}
		}
		// Now create any new types
		for _, eventType := range eventTypes {
			log.Infof("Creating new event type %s", eventType.Name)
			err = c.createEventType(tx, eventType)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *ConfigManager) createEventType(tx *utils.Tx, e types.EventType) error {
	// Since go interprets omitted as empty string for feature flag, translate empty string to NULL on insert.
	result, err := tx.Exec(
		"create_event_type",
		`INSERT INTO event_types (name, display_name, description, default_receiver_types, feature_flag)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		ON CONFLICT DO NOTHING`,
		e.Name,
		e.DisplayName,
		e.Description,
		pq.Array(e.DefaultReceiverTypes),
		e.FeatureFlag,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("Event already exists")
	}
	// Now add default configs for receivers
	_, err = tx.Exec(
		"set_receiver_defaults_for_new_event_type",
		`INSERT INTO receiver_event_types (receiver_id, event_type) (
			SELECT receiver_id, $1 FROM receivers
			WHERE receiver_type = ANY ($2)
		)`,
		e.Name,
		pq.Array(e.DefaultReceiverTypes),
	)
	return err
}

func (c *ConfigManager) updateEventType(tx *utils.Tx, e types.EventType) error {
	// Since go interprets omitted as empty string for feature flag, translate empty string to NULL on insert.
	result, err := tx.Exec(
		"update_event_type",
		`UPDATE event_types
		SET (display_name, description, default_receiver_types, feature_flag) = ($2, $3, $4, NULLIF($5, ''))
		WHERE name = $1`,
		e.Name,
		e.DisplayName,
		e.Description,
		pq.Array(e.DefaultReceiverTypes),
		e.FeatureFlag,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("Event type does not exist")
	}
	return nil
}

func (c *ConfigManager) deleteEventType(tx *utils.Tx, eventTypeName string) error {
	result, err := c.db.Exec(
		"delete_event_type",
		`DELETE FROM event_types
		WHERE name = $1`,
		eventTypeName,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("Event type does not exist")
	}
	return nil
}
