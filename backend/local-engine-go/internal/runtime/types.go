package runtime

import (
	"context"
	"time"
)

type LogSink func(line string)

type Instance struct {
	ID   string
	Host string
	Port int
}

type StartRequest struct {
	ImageID string
	DataDir string
	Name    string
	Mounts  []Mount
}

type ExecRequest struct {
	User  string
	Args  []string
	Env   map[string]string
	Dir   string
	Stdin *string
}

type Mount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
}

type Runtime interface {
	InitBase(ctx context.Context, imageID string, dataDir string) error
	ResolveImage(ctx context.Context, imageID string) (string, error)
	Start(ctx context.Context, req StartRequest) (Instance, error)
	Stop(ctx context.Context, id string) error
	Exec(ctx context.Context, id string, req ExecRequest) (string, error)
	WaitForReady(ctx context.Context, id string, timeout time.Duration) error
}
