package service

import (
	"time"

	"github.com/weaveworks/flux"
)

// InstanceID is a Weave Cloud instanceID.
type InstanceID string

type key string

// InstanceIDKey is the key against which we'll store the instance ID in contexts.
const InstanceIDKey key = "InstanceID"

// Status is the status of a given instance.
// TODO: How similar should this be to the `get-config` result?
type Status struct {
	Fluxsvc FluxsvcStatus `json:"fluxsvc" yaml:"fluxsvc"`
	Fluxd   FluxdStatus   `json:"fluxd" yaml:"fluxd"`
	Git     GitStatus     `json:"git" yaml:"git"`
}

// FluxsvcStatus contains the flux-api version.
type FluxsvcStatus struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// FluxdStatus is the status of a given flux daemon.
type FluxdStatus struct {
	Connected bool      `json:"connected" yaml:"connected"`
	Last      time.Time `json:"last,omitempty" yaml:"last,omitempty"`
	Version   string    `json:"version,omitempty" yaml:"version,omitempty"`
}

// GitStatus is the git configuration status of a given flux daemon.
type GitStatus struct {
	Configured bool           `json:"configured" yaml:"configured"`
	Error      string         `json:"error,omitempty" yaml:"error,omitempty"`
	Config     flux.GitConfig `json:"config"`
}
