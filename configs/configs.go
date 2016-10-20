package configs

// UserID is how users are identified.
type UserID string

// OrgID is how organizations are identified.
type OrgID string

// Subsystem is the name of a subsystem that has configuration. e.g. "deploy",
// "prism".
type Subsystem string

// Config is a configuration of a subsystem.
type Config map[string]interface{}
