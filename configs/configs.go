package configs

import (
	"time"
)

// UserID is how users are identified.
type UserID string

// OrgID is how organizations are identified.
type OrgID string

// Subsystem is the name of a subsystem that has configuration. e.g. "deploy",
// "cortex".
type Subsystem string

// Config is a configuration of a subsystem.
type Config map[string]interface{}

// CortexConfig is the configuration used by Cortex.
type CortexConfig struct {
	OrgID         OrgID             `json:"org_id"`
	LastEvaluated time.Time         `json:"last_evaluated"`
	RulesFiles    map[string]string `json:"rules_files"`
}
