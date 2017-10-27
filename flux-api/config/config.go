package config

import (
	"encoding/json"
)

// Notifier is the configuration for an event notifier.
type Notifier struct {
	HookURL         string `json:"hookURL" yaml:"hookURL"`
	Username        string `json:"username" yaml:"username"`
	ReleaseTemplate string `json:"releaseTemplate" yaml:"releaseTemplate"`
	// NotifyEvents should be a list of e.g. ["release", "sync"].
	NotifyEvents []string `json:"notifyEvents" yaml:"notifyEvents"`
}

// Instance is the configuration for a Weave Cloud instance.
type Instance struct {
	Slack Notifier `json:"slack" yaml:"slack"`
}

type untypedConfig map[string]interface{}

func (uc untypedConfig) toInstanceConfig() (Instance, error) {
	bytes, err := json.Marshal(uc)
	if err != nil {
		return Instance{}, err
	}
	var uic Instance
	if err := json.Unmarshal(bytes, &uic); err != nil {
		return Instance{}, err
	}
	return uic, nil
}

func (uic Instance) toUntypedConfig() (untypedConfig, error) {
	bytes, err := json.Marshal(uic)
	if err != nil {
		return nil, err
	}
	var uc untypedConfig
	if err := json.Unmarshal(bytes, &uc); err != nil {
		return nil, err
	}
	return uc, nil
}

// Patch is an alias of map[string]interface{}.
type Patch map[string]interface{}

// Patch patches an Instance config with the given Patch.
func (uic Instance) Patch(cp Patch) (Instance, error) {
	// Convert the strongly-typed config into an untyped form that's easier to patch
	uc, err := uic.toUntypedConfig()
	if err != nil {
		return Instance{}, err
	}

	applyPatch(uc, cp)

	// If the modifications detailed by the patch have resulted in JSON which
	// doesn't meet the config schema it will be caught here
	return uc.toInstanceConfig()
}

func applyPatch(uc untypedConfig, cp Patch) {
	for key, value := range cp {
		switch value := value.(type) {
		case nil:
			delete(uc, key)
		case map[string]interface{}:
			if uc[key] == nil {
				uc[key] = make(map[string]interface{})
			}
			if uc, ok := uc[key].(map[string]interface{}); ok {
				applyPatch(uc, value)
			}
		default:
			// Remaining types; []interface{}, bool, float64 & string
			// Note that we only support replacing arrays in their entirety
			// as there is no way to address subelements for removal or update
			uc[key] = value
		}
	}
}
