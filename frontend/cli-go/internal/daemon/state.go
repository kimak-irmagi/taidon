package daemon

import (
	"encoding/json"
	"os"

	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/util"
)

type EngineState struct {
	Endpoint   string `json:"endpoint"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"startedAt"`
	AuthToken  string `json:"authToken"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
}

func ReadEngineState(path string) (EngineState, error) {
	var state EngineState
	data, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func WriteEngineState(path string, state EngineState) error {
	data, _ := json.MarshalIndent(state, "", "  ")
	return util.AtomicWriteFile(path, data, 0o600)
}

func IsEngineStateStale(state EngineState, health client.HealthResponse, healthErr error, pidRunning bool) bool {
	if healthErr != nil {
		return true
	}
	if state.InstanceID != "" && health.InstanceID != "" && state.InstanceID != health.InstanceID {
		return true
	}
	if state.PID > 0 && !pidRunning {
		return true
	}
	return false
}
