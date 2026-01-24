package run

type Event struct {
	Type       string        `json:"type"`
	Ts         string        `json:"ts"`
	InstanceID string        `json:"instance_id,omitempty"`
	Data       string        `json:"data,omitempty"`
	ExitCode   *int          `json:"exit_code,omitempty"`
	Error      *ErrorPayload `json:"error,omitempty"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
