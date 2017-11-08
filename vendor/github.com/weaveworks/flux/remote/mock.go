package remote

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/guid"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/job"
	"github.com/weaveworks/flux/update"
)

type MockPlatform struct {
	PingError error

	VersionAnswer string
	VersionError  error

	ExportAnswer []byte
	ExportError  error

	ListServicesAnswer []flux.ControllerStatus
	ListServicesError  error

	ListImagesAnswer []flux.ImageStatus
	ListImagesError  error

	UpdateManifestsArgTest func(update.Spec) error
	UpdateManifestsAnswer  job.ID
	UpdateManifestsError   error

	SyncNotifyError error

	SyncStatusAnswer []string
	SyncStatusError  error

	JobStatusAnswer job.Status
	JobStatusError  error

	GitRepoConfigAnswer flux.GitConfig
	GitRepoConfigError  error
}

func (p *MockPlatform) Ping(ctx context.Context) error {
	return p.PingError
}

func (p *MockPlatform) Version(ctx context.Context) (string, error) {
	return p.VersionAnswer, p.VersionError
}

func (p *MockPlatform) Export(ctx context.Context) ([]byte, error) {
	return p.ExportAnswer, p.ExportError
}

func (p *MockPlatform) ListServices(ctx context.Context, ns string) ([]flux.ControllerStatus, error) {
	return p.ListServicesAnswer, p.ListServicesError
}

func (p *MockPlatform) ListImages(context.Context, update.ResourceSpec) ([]flux.ImageStatus, error) {
	return p.ListImagesAnswer, p.ListImagesError
}

func (p *MockPlatform) UpdateManifests(ctx context.Context, s update.Spec) (job.ID, error) {
	if p.UpdateManifestsArgTest != nil {
		if err := p.UpdateManifestsArgTest(s); err != nil {
			return job.ID(""), err
		}
	}
	return p.UpdateManifestsAnswer, p.UpdateManifestsError
}

func (p *MockPlatform) SyncNotify(ctx context.Context) error {
	return p.SyncNotifyError
}

func (p *MockPlatform) SyncStatus(context.Context, string) ([]string, error) {
	return p.SyncStatusAnswer, p.SyncStatusError
}

func (p *MockPlatform) JobStatus(context.Context, job.ID) (job.Status, error) {
	return p.JobStatusAnswer, p.JobStatusError
}

func (p *MockPlatform) GitRepoConfig(ctx context.Context, regenerate bool) (flux.GitConfig, error) {
	return p.GitRepoConfigAnswer, p.GitRepoConfigError
}

var _ Platform = &MockPlatform{}

// -- Battery of tests for a platform mechanism. Since these
// essentially wrap the platform in various transports, we expect
// arguments and answers to be preserved.

func PlatformTestBattery(t *testing.T, wrap func(mock Platform) Platform) {
	// set up
	namespace := "the-space-of-names"
	serviceID := flux.MustParseResourceID(namespace + "/service")
	serviceList := []flux.ResourceID{serviceID}
	services := flux.ResourceIDSet{}
	services.Add(serviceList)

	now := time.Now().UTC()

	imageID, _ := image.ParseRef("quay.io/example.com/frob:v0.4.5")
	serviceAnswer := []flux.ControllerStatus{
		flux.ControllerStatus{
			ID:     flux.MustParseResourceID("foobar/hello"),
			Status: "ok",
			Containers: []flux.Container{
				flux.Container{
					Name: "frobnicator",
					Current: image.Info{
						ID:        imageID,
						CreatedAt: now,
					},
				},
			},
		},
		flux.ControllerStatus{},
	}

	imagesAnswer := []flux.ImageStatus{
		flux.ImageStatus{
			ID: flux.MustParseResourceID("barfoo/yello"),
			Containers: []flux.Container{
				{
					Name: "flubnicator",
					Current: image.Info{
						ID: imageID,
					},
				},
			},
		},
	}

	syncStatusAnswer := []string{
		"commit 1",
		"commit 2",
		"commit 3",
	}

	updateSpec := update.Spec{
		Type: update.Images,
		Spec: update.ReleaseSpec{
			ServiceSpecs: []update.ResourceSpec{
				update.ResourceSpecAll,
			},
			ImageSpec: update.ImageSpecLatest,
		},
	}
	checkUpdateSpec := func(s update.Spec) error {
		if !reflect.DeepEqual(updateSpec, s) {
			return errors.New("expected != actual")
		}
		return nil
	}

	mock := &MockPlatform{
		ListServicesAnswer:     serviceAnswer,
		ListImagesAnswer:       imagesAnswer,
		UpdateManifestsArgTest: checkUpdateSpec,
		UpdateManifestsAnswer:  job.ID(guid.New()),
		SyncStatusAnswer:       syncStatusAnswer,
	}

	ctx := context.Background()

	// OK, here we go
	client := wrap(mock)

	if err := client.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	ss, err := client.ListServices(ctx, namespace)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ss, mock.ListServicesAnswer) {
		t.Error(fmt.Errorf("expected:\n%#v\ngot:\n%#v", mock.ListServicesAnswer, ss))
	}
	mock.ListServicesError = fmt.Errorf("list services query failure")
	ss, err = client.ListServices(ctx, namespace)
	if err == nil {
		t.Error("expected error from ListServices, got nil")
	}

	ims, err := client.ListImages(ctx, update.ResourceSpecAll)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(ims, mock.ListImagesAnswer) {
		t.Error(fmt.Errorf("expected:\n%#v\ngot:\n%#v", mock.ListImagesAnswer, ims))
	}
	mock.ListImagesError = fmt.Errorf("list images error")
	if _, err = client.ListImages(ctx, update.ResourceSpecAll); err == nil {
		t.Error("expected error from ListImages, got nil")
	}

	jobid, err := mock.UpdateManifests(ctx, updateSpec)
	if err != nil {
		t.Error(err)
	}
	if jobid != mock.UpdateManifestsAnswer {
		t.Error(fmt.Errorf("expected %q, got %q", mock.UpdateManifestsAnswer, jobid))
	}
	mock.UpdateManifestsError = fmt.Errorf("update manifests error")
	if _, err = client.UpdateManifests(ctx, updateSpec); err == nil {
		t.Error("expected error from UpdateManifests, got nil")
	}

	if err := client.SyncNotify(ctx); err != nil {
		t.Error(err)
	}

	syncSt, err := client.SyncStatus(ctx, "HEAD")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(mock.SyncStatusAnswer, syncSt) {
		t.Error(fmt.Errorf("expected: %#v\ngot: %#v"), mock.SyncStatusAnswer, syncSt)
	}
}
