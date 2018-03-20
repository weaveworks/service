// +build integration

package sql

import (
	"errors"
	"net/url"
	"testing"

	"github.com/weaveworks/service/flux-api/config"
	"github.com/weaveworks/service/flux-api/db"
	"github.com/weaveworks/service/flux-api/instance"
	"github.com/weaveworks/service/flux-api/service"
)

var (
	dbURL = "postgres://postgres@postgres:5432?sslmode=disable"
)

func newDB(t *testing.T) *DB {
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Migrate(dbURL, "../../db/migrations/postgres"); err != nil {
		t.Fatal(err)
	}
	db, err := New(u.Scheme, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpdateOK(t *testing.T) {
	db := newDB(t)

	inst := service.InstanceID("floaty-womble-abc123")
	c := instance.Config{
		Settings: config.Instance{
			Slack: config.Notifier{
				Username: "test Slack user",
			},
		},
	}
	err := db.UpdateConfig(inst, func(_ instance.Config) (instance.Config, error) {
		return c, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c1, err := db.GetConfig(inst)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Settings.Slack.Username != c.Settings.Slack.Username {
		t.Errorf("expected config %#v, got %#v", c.Settings, c1.Settings)
	}
}

func TestUpdateRollback(t *testing.T) {
	db := newDB(t)

	inst := service.InstanceID("floaty-womble-abc123")
	c := instance.Config{
		Settings: config.Instance{
			Slack: config.Notifier{
				Username: "test Slack user",
			},
		},
	}

	err := db.UpdateConfig(inst, func(_ instance.Config) (instance.Config, error) {
		return c, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.UpdateConfig(inst, func(_ instance.Config) (instance.Config, error) {
		return instance.Config{}, errors.New("arbitrary fail")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	c1, err := db.GetConfig(inst)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Settings.Slack.Username != c.Settings.Slack.Username {
		t.Errorf("expected config %#v, got %#v", c.Settings, c1.Settings)
	}
}
