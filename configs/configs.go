package configs

import (
	"fmt"
	"reflect"
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

// ToCortexConfig turns a Config into a CortexConfig, if possible.
func (c *Config) ToCortexConfig() (*CortexConfig, error) {
	m := map[string]interface{}(*c)
	var cc CortexConfig
	orgID, ok := m["org_id"]
	if ok {
		switch v := orgID.(type) {
		case string:
			cc.OrgID = OrgID(v)
		default:
			return nil, fmt.Errorf("Not a CortexConfig, wrong type (%v) for org_id: %v", reflect.TypeOf(orgID), c)
		}
	}
	lastEvaluated, ok := m["last_evaluated"]
	if ok {
		switch v := lastEvaluated.(type) {
		case string:
			t, err := time.Parse(time.RFC3339Nano, v)
			if err != nil {
				return nil, fmt.Errorf("Not a CortexConfig, could not parse last_evaluated: %v", err)
			}
			cc.LastEvaluated = t
		case time.Time:
			cc.LastEvaluated = v
		default:
			return nil, fmt.Errorf("Not a CortexConfig, wrong type (%v) for last_evaluated: %v", reflect.TypeOf(lastEvaluated), c)
		}
	}
	rulesFiles, ok := m["rules_files"]
	if ok {
		switch v := rulesFiles.(type) {
		case map[string]string:
			cc.RulesFiles = v
		case map[string]interface{}:
			cc.RulesFiles = map[string]string{}
			for key, value := range v {
				switch valueStr := value.(type) {
				case string:
					cc.RulesFiles[key] = valueStr
				default:
					return nil, fmt.Errorf("Could not convert rules file %s to string: %v", key, value)
				}
			}
		default:
			return nil, fmt.Errorf("Not a CortexConfig, wrong type (%v) for rules_files: %v", reflect.TypeOf(rulesFiles), c)
		}
	}
	return &cc, nil
}

// CortexConfig is the configuration used by Cortex.
type CortexConfig struct {
	OrgID         OrgID             `json:"org_id"`
	LastEvaluated time.Time         `json:"last_evaluated"`
	RulesFiles    map[string]string `json:"rules_files"`
}

// ToConfig converts a CortexConfig to a general Config.
func (cc *CortexConfig) ToConfig() Config {
	return Config{
		"org_id":         cc.OrgID,
		"last_evaluated": cc.LastEvaluated,
		"rules_files":    cc.RulesFiles,
	}
}
