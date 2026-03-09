package dbms

import (
	"context"

	"github.com/sqlrs/engine-local/internal/runtime"
)

type Connector interface {
	PrepareSnapshot(ctx context.Context, instance runtime.Instance) error
	ResumeSnapshot(ctx context.Context, instance runtime.Instance) error
}
