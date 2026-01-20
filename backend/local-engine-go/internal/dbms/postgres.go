package dbms

import (
	"context"
	"fmt"

	"sqlrs/engine/internal/runtime"
)

type PostgresConnector struct {
	Runtime runtime.Runtime
}

func NewPostgres(runtime runtime.Runtime) *PostgresConnector {
	return &PostgresConnector{Runtime: runtime}
}

func (c *PostgresConnector) PrepareSnapshot(ctx context.Context, instance runtime.Instance) error {
	if c == nil || c.Runtime == nil {
		return fmt.Errorf("runtime is required")
	}
	_, err := c.Runtime.Exec(ctx, instance.ID, runtime.ExecRequest{
		User: "postgres",
		Args: []string{"pg_ctl", "-D", "/var/lib/postgresql/data", "-m", "fast", "-w", "stop"},
	})
	return err
}

func (c *PostgresConnector) ResumeSnapshot(ctx context.Context, instance runtime.Instance) error {
	if c == nil || c.Runtime == nil {
		return fmt.Errorf("runtime is required")
	}
	_, err := c.Runtime.Exec(ctx, instance.ID, runtime.ExecRequest{
		User: "postgres",
		Args: []string{
			"pg_ctl", "-D", "/var/lib/postgresql/data",
			"-o", "-c listen_addresses=* -p 5432",
			"-w", "start",
		},
	})
	return err
}
