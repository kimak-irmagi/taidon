package dbms

import (
	"context"

	"sqlrs/engine/internal/runtime"
)

type Connector interface {
	PrepareSnapshot(ctx context.Context, instance runtime.Instance) error
	ResumeSnapshot(ctx context.Context, instance runtime.Instance) error
}
