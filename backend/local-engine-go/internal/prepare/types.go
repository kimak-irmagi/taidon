package prepare

type Request struct {
	PrepareKind string   `json:"prepare_kind"`
	ImageID     string   `json:"image_id"`
	PsqlArgs    []string `json:"psql_args"`
	Stdin       *string  `json:"stdin,omitempty"`
}

type Accepted struct {
	JobID     string `json:"job_id"`
	StatusURL string `json:"status_url"`
	EventsURL string `json:"events_url,omitempty"`
	Status    string `json:"status,omitempty"`
}

type Status struct {
	JobID       string         `json:"job_id"`
	Status      string         `json:"status"`
	PrepareKind string         `json:"prepare_kind"`
	ImageID     string         `json:"image_id"`
	CreatedAt   *string        `json:"created_at,omitempty"`
	StartedAt   *string        `json:"started_at,omitempty"`
	FinishedAt  *string        `json:"finished_at,omitempty"`
	Result      *Result        `json:"result,omitempty"`
	Error       *ErrorResponse `json:"error,omitempty"`
}

type Event struct {
	Type    string         `json:"type"`
	Ts      string         `json:"ts"`
	Status  string         `json:"status,omitempty"`
	Message string         `json:"message,omitempty"`
	Result  *Result        `json:"result,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

type Result struct {
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
