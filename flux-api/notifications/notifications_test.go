package notifications

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/event"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/update"
	"github.com/weaveworks/service/flux-api/service"
)

// Generate an example release
func exampleRelease(t *testing.T) *event.ReleaseEventMetadata {
	img1a1, _ := image.ParseRef("img1:a1")
	img1a2, _ := image.ParseRef("img1:a2")
	exampleResult := update.Result{
		flux.MustParseResourceID("default/helloworld"): {
			Status: update.ReleaseStatusFailed,
			Error:  "overall-release-error",
			PerContainer: []update.ContainerUpdate{
				{
					Container: "container1",
					Current:   img1a1,
					Target:    img1a2,
				},
			},
		},
	}
	return &event.ReleaseEventMetadata{
		Cause: update.Cause{
			User:    "test-user",
			Message: "this was to test notifications",
		},
		Spec: update.ReleaseSpec{
			ServiceSpecs: []update.ResourceSpec{update.ResourceSpec("default/helloworld")},
			ImageSpec:    update.ImageSpecLatest,
			Kind:         update.ReleaseKindExecute,
			Excludes:     nil,
		},
		ReleaseEventCommon: event.ReleaseEventCommon{
			Result: exampleResult,
		},
	}
}

func TestRelease_DryRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Expected no http request to have been made")
	}))
	defer server.Close()

	// It should send releases to slack
	r := exampleRelease(t)
	ev := event.Event{Metadata: r, Type: event.EventRelease}
	r.Spec.Kind = update.ReleaseKindPlan
	if err := Event(server.URL, ev, ""); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationEventsURL(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.WriteHeader(200)
	}))
	defer server.Close()

	var instanceID service.InstanceID = "2"
	eventsURL := server.URL + fmt.Sprintf("/api/notification/slack/%v/{eventType}", instanceID)
	eventType := "deploy"
	expectedPath := fmt.Sprintf("/api/notification/slack/%v/%v", instanceID, eventType)

	ev := event.Event{Metadata: exampleRelease(t), Type: event.EventRelease}

	err := Event(eventsURL, ev, "")

	if err != nil {
		t.Fatal(err)
	}

	if gotReq.URL.EscapedPath() != expectedPath {
		t.Fatalf("Expected: %v, Got: %v", expectedPath, gotReq.URL.String())
	}
}
