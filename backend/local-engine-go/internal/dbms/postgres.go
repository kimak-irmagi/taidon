package dbms

import (
	"context"
	"fmt"
	"log"
	"strings"

	"sqlrs/engine/internal/runtime"
)

type PostgresConnector struct {
	Runtime  runtime.Runtime
	logLevel func() string
}

type PostgresOption func(*PostgresConnector)

func WithLogLevel(fn func() string) PostgresOption {
	return func(connector *PostgresConnector) {
		connector.logLevel = fn
	}
}

func NewPostgres(runtime runtime.Runtime, opts ...PostgresOption) *PostgresConnector {
	connector := &PostgresConnector{Runtime: runtime}
	for _, opt := range opts {
		if opt != nil {
			opt(connector)
		}
	}
	return connector
}

func (c *PostgresConnector) PrepareSnapshot(ctx context.Context, instance runtime.Instance) error {
	if c == nil || c.Runtime == nil {
		return fmt.Errorf("runtime is required")
	}
	if c.logInfoEnabled() {
		log.Printf("pg_ctl stop start instance=%s", instance.ID)
	}
	output, err := c.Runtime.Exec(ctx, instance.ID, runtime.ExecRequest{
		User: "postgres",
		Args: []string{"pg_ctl", "-D", runtime.PostgresDataDir, "-m", "fast", "-w", "stop"},
	})
	if c.logInfoEnabled() {
		log.Printf("pg_ctl stop result instance=%s err=%v output=%q", instance.ID, err, strings.TrimSpace(output))
	}
	if err == nil {
		if verifyErr := c.verifyStopped(ctx, instance); verifyErr != nil {
			if c.logInfoEnabled() {
				log.Printf("pg_ctl stop verify failed instance=%s err=%v", instance.ID, verifyErr)
			}
			return verifyErr
		}
	}
	return err
}

func (c *PostgresConnector) ResumeSnapshot(ctx context.Context, instance runtime.Instance) error {
	if c == nil || c.Runtime == nil {
		return fmt.Errorf("runtime is required")
	}
	if c.logInfoEnabled() {
		log.Printf("pg_ctl start start instance=%s", instance.ID)
	}
	output, err := c.Runtime.Exec(ctx, instance.ID, runtime.ExecRequest{
		User: "postgres",
		Args: []string{
			"pg_ctl", "-D", runtime.PostgresDataDir,
			"-o", "-c listen_addresses=* -p 5432",
			"-w", "start",
		},
	})
	if c.logInfoEnabled() {
		log.Printf("pg_ctl start result instance=%s err=%v output=%q", instance.ID, err, strings.TrimSpace(output))
	}
	return err
}

func (c *PostgresConnector) verifyStopped(ctx context.Context, instance runtime.Instance) error {
	output, err := c.Runtime.Exec(ctx, instance.ID, runtime.ExecRequest{
		User: "postgres",
		Args: []string{
			"pg_ctl",
			"-D", runtime.PostgresDataDir,
			"status",
		},
	})
	if err == nil {
		msg := strings.TrimSpace(output)
		if msg == "" {
			msg = "pg_ctl status returned running"
		}
		return fmt.Errorf("postgres still running: %s", msg)
	}
	lower := strings.ToLower(output)
	if strings.Contains(lower, "no server running") || strings.Contains(lower, "not running") {
		return nil
	}
	msg := strings.TrimSpace(output)
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("cannot verify postgres stopped: %s", msg)
}

func (c *PostgresConnector) logInfoEnabled() bool {
	level := "debug"
	if c != nil && c.logLevel != nil {
		value := strings.TrimSpace(strings.ToLower(c.logLevel()))
		if value != "" {
			level = value
		}
	}
	switch level {
	case "debug", "info":
		return true
	default:
		return false
	}
}
