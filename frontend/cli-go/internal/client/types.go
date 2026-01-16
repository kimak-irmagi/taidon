package client

type HealthResponse struct {
	Ok         bool   `json:"ok"`
	Version    string `json:"version"`
	InstanceID string `json:"instanceId"`
	PID        int    `json:"pid"`
}

type ListFilters struct {
	Name     string
	Instance string
	State    string
	Kind     string
	Image    string
}

type NameEntry struct {
	Name             string  `json:"name"`
	InstanceID       *string `json:"instance_id,omitempty"`
	ImageID          string  `json:"image_id"`
	StateID          string  `json:"state_id"`
	StateFingerprint string  `json:"state_fingerprint,omitempty"`
	Status           string  `json:"status"`
	LastUsedAt       *string `json:"last_used_at,omitempty"`
}

type InstanceEntry struct {
	InstanceID string  `json:"instance_id"`
	ImageID    string  `json:"image_id"`
	StateID    string  `json:"state_id"`
	Name       *string `json:"name,omitempty"`
	CreatedAt  string  `json:"created_at"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	Status     string  `json:"status"`
}

type StateEntry struct {
	StateID     string `json:"state_id"`
	ImageID     string `json:"image_id"`
	PrepareKind string `json:"prepare_kind"`
	PrepareArgs string `json:"prepare_args_normalized"`
	CreatedAt   string `json:"created_at"`
	SizeBytes   *int64 `json:"size_bytes,omitempty"`
	RefCount    int    `json:"refcount"`
}

type PrepareJobRequest struct {
	PrepareKind string   `json:"prepare_kind"`
	ImageID     string   `json:"image_id"`
	PsqlArgs    []string `json:"psql_args"`
	Stdin       *string  `json:"stdin,omitempty"`
}

type PrepareJobAccepted struct {
	JobID     string `json:"job_id"`
	StatusURL string `json:"status_url"`
	EventsURL string `json:"events_url,omitempty"`
	Status    string `json:"status,omitempty"`
}

type PrepareJobStatus struct {
	JobID       string            `json:"job_id"`
	Status      string            `json:"status"`
	PrepareKind string            `json:"prepare_kind"`
	ImageID     string            `json:"image_id"`
	CreatedAt   *string           `json:"created_at,omitempty"`
	StartedAt   *string           `json:"started_at,omitempty"`
	FinishedAt  *string           `json:"finished_at,omitempty"`
	Result      *PrepareJobResult `json:"result,omitempty"`
	Error       *ErrorResponse    `json:"error,omitempty"`
}

type PrepareJobEvent struct {
	Type    string            `json:"type"`
	Ts      string            `json:"ts"`
	Status  string            `json:"status,omitempty"`
	Message string            `json:"message,omitempty"`
	Result  *PrepareJobResult `json:"result,omitempty"`
	Error   *ErrorResponse    `json:"error,omitempty"`
}

type PrepareJobResult struct {
	DSN                   string `json:"dsn"`
	InstanceID            string `json:"instance_id"`
	StateID               string `json:"state_id"`
	ImageID               string `json:"image_id"`
	PrepareKind           string `json:"prepare_kind"`
	PrepareArgsNormalized string `json:"prepare_args_normalized"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
