package main

import (
	"io"
	"io/ioutil"
	"net/http"

	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"github.com/weaveworks/service/users"
)

// ifEmpty(a,b) returns b iff a is empty
func ifEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func newLauncherServiceLogger(usersClient users.UsersClient) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		var orgID string

		externalID := r.URL.Query().Get("instanceID")
		if externalID != "" {
			ctx := r.Context()
			// Lookup the internal OrgID
			response, err := usersClient.GetOrganization(ctx, &users.GetOrganizationRequest{
				ID: &users.GetOrganizationRequest_ExternalID{ExternalID: externalID},
			})
			if err != nil {
				user.LogWith(ctx, logging.Global()).Errorf("launcherServiceLogger: Failed to lookup externalID: %s", externalID)
			} else {
				orgID = response.Organization.ID
			}
		}

		event := Event{
			ID:             r.URL.Path,
			Product:        "launcher-service",
			UserAgent:      r.UserAgent(),
			OrganizationID: orgID,
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newProbeRequestLogger() HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		orgID, err := user.ExtractOrgID(r.Context())
		if err != nil {
			return Event{}, false
		}

		event := Event{
			ID:             r.URL.Path,
			Product:        "scope-probe",
			Version:        r.Header.Get(probeVersionHeader),
			UserAgent:      r.UserAgent(),
			ClientID:       r.Header.Get(probeIDHeader),
			OrganizationID: orgID,
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newUIRequestLogger(userIDHeader string) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		sessionCookie, err := r.Cookie(sessionCookieKey)
		var sessionID string
		if err == nil {
			sessionID = sessionCookie.Value
		}

		orgID, _ := user.ExtractOrgID(r.Context())

		event := Event{
			ID:             r.URL.Path,
			SessionID:      sessionID,
			Product:        "scope-ui",
			UserAgent:      r.UserAgent(),
			OrganizationID: orgID,
			UserID:         r.Header.Get(userIDHeader),
			IPAddress:      mustSplitHostname(r),
		}
		return event, true
	}
}

func newAnalyticsLogger(userIDHeader string) HTTPEventExtractor {
	return func(r *http.Request) (Event, bool) {
		sessionCookie, err := r.Cookie(sessionCookieKey)
		var sessionID string
		if err == nil {
			sessionID = sessionCookie.Value
		}

		values, err := ioutil.ReadAll(&io.LimitedReader{
			R: r.Body,
			N: maxAnalyticsPayloadSize,
		})
		if err != nil {
			return Event{}, false
		}

		event := Event{
			ID:        r.URL.Path,
			SessionID: sessionID,
			Product:   "scope-ui",
			UserAgent: r.UserAgent(),
			UserID:    r.Header.Get(userIDHeader),
			Values:    string(values),
			IPAddress: mustSplitHostname(r),
		}
		return event, true
	}
}
