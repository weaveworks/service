package sender

import (
	"encoding/json"
	"sync"

	"context"

	googleLogging "cloud.google.com/go/logging"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/weaveworks/service/notification-eventmanager/types"
	googleOAuth2 "golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	grpcCodes "google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

// StackdriverSender contains logID to write to and map of receivers
type StackdriverSender struct {
	// LogID is the ID of the log to write to
	LogID string

	// Clients is a map of stackdriver clients from PrivateKeyID to client
	Clients map[string]*googleLogging.Client
	mu      sync.Mutex
}

// serviceFile struct is using to get private_key_id and project_id from service file
type serviceFile struct {
	PrivateKeyID string `json:"private_key_id"`
	ProjectID    string `json:"project_id"`
}

func (sd *StackdriverSender) getClientByServiceFile(ctx context.Context, content json.RawMessage) (*googleLogging.Client, error) {
	var sf serviceFile
	if err := json.Unmarshal(content, &sf); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal service file content")
	}

	if sf.PrivateKeyID == "" || sf.ProjectID == "" {
		return nil, errors.Errorf("private_key_id and project_id cannot be empty in service file")
	}

	sd.mu.Lock()
	defer sd.mu.Unlock()

	if client, ok := sd.Clients[sf.PrivateKeyID]; ok {
		return client, nil
	}

	// JWTConfigFromJSON uses a Google Developers service account JSON key file to read
	// the credentials that authorize and authenticate the requests.
	cfg, err := googleOAuth2.JWTConfigFromJSON(content, googleLogging.WriteScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read the credentials from Google Developers service account JSON key file")
	}

	// Creates a new logging client associated with the provided token source from service file
	client, err := googleLogging.NewClient(ctx, sf.ProjectID, option.WithTokenSource(cfg.TokenSource(ctx)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create google cloud client")
	}
	sd.Clients[sf.PrivateKeyID] = client

	client.OnError = func(e error) {
		log.Debugf("stackdriver logging: %v", e)
	}

	return client, nil
}

// Send sends data to Stackdriver with creds from addr
func (sd *StackdriverSender) Send(ctx context.Context, addr json.RawMessage, notif types.Notification, _ string) error {
	client, err := sd.getClientByServiceFile(ctx, addr)
	if err != nil {
		return errors.Wrapf(err, "cannot get stackdriver client for service file")
	}

	// Selects the log to write to.
	logger := client.Logger(sd.LogID)
	var entry googleLogging.Entry
	if err := json.Unmarshal(notif.Data, &entry); err != nil {
		return errors.Wrapf(err, "cannot unmarshal stackdriver data %s", notif.Data)
	}

	// log the entry synchronously without any buffering.
	// Because of if use non-blocking Log() we may lose some notifications during client reconnecting
	if err := logger.LogSync(ctx, entry); err != nil {
		// Retry only for 429, 500 and 503 server errors, https://godoc.org/google.golang.org/genproto/googleapis/rpc/code
		status, ok := grpcStatus.FromError(err)
		if ok {
			switch status.Code() {
			case grpcCodes.Unavailable, grpcCodes.Unknown, grpcCodes.DataLoss, grpcCodes.ResourceExhausted:
				return RetriableError{errors.Wrap(err, "internal server error logging to stackdriver")}
			}
		}
		return errors.Wrap(err, "error logging to stackdriver")
	}

	return nil
}

// Stop calls Client.Close before program exits to flush any buffered log entries to the Stackdriver Logging service.
func (sd *StackdriverSender) Stop() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	for _, cl := range sd.Clients {
		log.Debugf("closing stackdriver client")
		if err := cl.Close(); err != nil {
			log.Errorf("failed to close stackdriver client: %s", err)
		}
	}
}
