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
	StateID             string `json:"state_id"`
	ImageID             string `json:"image_id"`
	PrepareKind         string `json:"prepare_kind"`
	PrepareArgs         string `json:"prepare_args_normalized"`
	CreatedAt           string `json:"created_at"`
	SizeBytes           *int64 `json:"size_bytes,omitempty"`
	RefCount            int    `json:"refcount"`
}
