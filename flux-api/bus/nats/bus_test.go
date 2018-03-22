// +build integration

package nats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/weaveworks/flux/api"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/service/flux-api/service"
)

func setup(t *testing.T) *NATS {
	bus, err := NewMessageBus("nats://nats:4222")
	if err != nil {
		t.Fatal(err)
	}
	return bus
}

func subscribe(ctx context.Context, t *testing.T, bus *NATS, inst service.InstanceID, server api.UpstreamServer, errc chan error) {
	bus.Subscribe(ctx, inst, server, errc)
	if err := bus.AwaitPresence(inst, 5*time.Second); err != nil {
		t.Fatal("Timed out waiting for instance to subscribe")
	}
}

func TestPing(t *testing.T) {
	bus := setup(t)
	errc := make(chan error)
	instID := service.InstanceID("wirey-bird-68")
	platA := &remote.MockServer{}

	ctx := context.Background()
	subscribe(ctx, t, bus, instID, platA, errc)

	// AwaitPresence uses Ping, so we have to install our error after
	// subscribe succeeds.
	platA.PingError = remote.FatalError{Err: errors.New("ping problem")}
	if err := platA.Ping(ctx); err == nil {
		t.Fatalf("expected error from directly calling ping, got nil")
	}

	err := bus.Ping(ctx, instID)
	if err == nil {
		t.Errorf("expected error from ping, got nil")
	} else if err.Error() != "ping problem" {
		t.Errorf("got the wrong error: %s", err.Error())
	}

	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("expected error return from subscription but didn't get one")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected error return from subscription but didn't get one")
	}
}

func TestMethods(t *testing.T) {
	bus := setup(t)
	errc := make(chan error, 1)
	instA := service.InstanceID("steamy-windows-89")

	ctx := context.Background()

	wrap := func(mock api.UpstreamServer) api.UpstreamServer {
		subscribe(ctx, t, bus, instA, mock, errc)
		plat, err := bus.Connect(instA)
		if err != nil {
			t.Fatal(err)
		}
		return plat
	}
	remote.ServerTestBattery(t, wrap)

	close(errc)
	err := <-errc
	if err != nil {
		t.Fatalf("expected nil from subscription channel, but got err %v", err)
	}
}

// A fatal error (a problem with the RPC connection, rather than a
// problem with processing the request) should both disconnect the RPC
// server, and be returned to the caller, all the way across the bus.
func TestFatalErrorDisconnects(t *testing.T) {
	bus := setup(t)

	ctx := context.Background()
	errc := make(chan error)

	instA := service.InstanceID("golden-years-75")
	mockA := &remote.MockServer{
		ListServicesError: remote.FatalError{Err: errors.New("disaster")},
	}
	subscribe(ctx, t, bus, instA, mockA, errc)

	plat, err := bus.Connect(instA)
	if err != nil {
		t.Fatal(err)
	}

	_, err = plat.ListServices(ctx, "")
	if err == nil {
		t.Error("expected error, got nil")
	}

	select {
	case err = <-errc:
		if err == nil {
			t.Error("expected error from subscription being killed, got nil")
		}
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for expected error from subscription closing")
	}
}

func TestNewConnectionKicks(t *testing.T) {
	bus := setup(t)

	instA := service.InstanceID("breaky-chain-77")

	mockA := &remote.MockServer{}
	errA := make(chan error)
	ctx := context.Background()
	subscribe(ctx, t, bus, instA, mockA, errA)

	mockB := &remote.MockServer{}
	errB := make(chan error)
	subscribe(ctx, t, bus, instA, mockB, errB)

	select {
	case <-errA:
		break
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for connection to be kicked")
	}

	close(errB)
	err := <-errB
	if err != nil {
		t.Errorf("expected no error from second connection, but got %q", err)
	}
}
