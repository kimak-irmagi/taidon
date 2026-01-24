package run

import (
	"context"
	"fmt"
	"strings"

	"sqlrs/engine/internal/registry"
	engineRuntime "sqlrs/engine/internal/runtime"
)

const (
	kindPsql    = "psql"
	kindPgbench = "pgbench"
)

type Options struct {
	Registry *registry.Registry
	Runtime  engineRuntime.Runtime
}

type Manager struct {
	registry *registry.Registry
	runtime  engineRuntime.Runtime
}

type Request struct {
	InstanceRef string   `json:"instance_ref"`
	Kind        string   `json:"kind"`
	Command     *string  `json:"command,omitempty"`
	Args        []string `json:"args"`
	Stdin       *string  `json:"stdin,omitempty"`
	Steps       []Step   `json:"steps,omitempty"`
}

type Step struct {
	Args  []string `json:"args"`
	Stdin *string  `json:"stdin,omitempty"`
}

type Result struct {
	InstanceID string
	Stdout     string
	Stderr     string
	ExitCode   int
}

func NewManager(opts Options) (*Manager, error) {
	if opts.Registry == nil {
		return nil, fmt.Errorf("registry is required")
	}
	if opts.Runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	return &Manager{
		registry: opts.Registry,
		runtime:  opts.Runtime,
	}, nil
}

func (m *Manager) Run(ctx context.Context, req Request) (Result, error) {
	if strings.TrimSpace(req.InstanceRef) == "" {
		return Result{}, ValidationError{Message: "instance_ref is required"}
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if !isKnownKind(kind) {
		return Result{}, ValidationError{Message: "unknown run kind"}
	}
	entry, ok, _, err := m.registry.GetInstance(ctx, req.InstanceRef)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, NotFoundError{Message: "instance not found"}
	}
	runtimeID := ""
	if entry.RuntimeID != nil {
		runtimeID = strings.TrimSpace(*entry.RuntimeID)
	}
	if runtimeID == "" {
		return Result{}, ConflictError{Message: "instance runtime id is missing"}
	}

	args := append([]string{}, req.Args...)
	if kind == kindPsql && hasPsqlConnectionArgs(args) {
		return Result{}, ConflictError{Message: "conflicting psql connection arguments"}
	}
	if kind == kindPgbench && hasPgbenchConnectionArgs(args) {
		return Result{}, ConflictError{Message: "conflicting pgbench connection arguments"}
	}

	command := ""
	if req.Command != nil {
		command = strings.TrimSpace(*req.Command)
	}
	if command == "" {
		command = defaultCommand(kind)
	}
	if command == "" {
		return Result{}, ValidationError{Message: "command is required"}
	}

	if len(req.Steps) > 0 {
		if kind != kindPsql {
			return Result{}, ValidationError{Message: "steps are only supported for run:psql"}
		}
		if len(args) > 0 {
			return Result{}, ValidationError{Message: "steps cannot be combined with args"}
		}
		if req.Stdin != nil {
			return Result{}, ValidationError{Message: "steps cannot be combined with stdin"}
		}

		var output strings.Builder
		for _, step := range req.Steps {
			stepArgs := append([]string{}, step.Args...)
			if hasPsqlConnectionArgs(stepArgs) {
				return Result{}, ConflictError{Message: "conflicting psql connection arguments"}
			}
			execArgs := buildExecArgs(kind, command, stepArgs)
			stepOutput, err := m.runtime.Exec(ctx, runtimeID, engineRuntime.ExecRequest{
				User:  "postgres",
				Args:  execArgs,
				Stdin: step.Stdin,
			})
			if err != nil {
				return Result{}, fmt.Errorf("exec failed: %w", err)
			}
			output.WriteString(stepOutput)
		}
		return Result{
			InstanceID: entry.InstanceID,
			Stdout:     output.String(),
			ExitCode:   0,
		}, nil
	}

	execArgs := buildExecArgs(kind, command, args)
	output, err := m.runtime.Exec(ctx, runtimeID, engineRuntime.ExecRequest{
		User:  "postgres",
		Args:  execArgs,
		Stdin: req.Stdin,
	})
	if err != nil {
		return Result{}, fmt.Errorf("exec failed: %w", err)
	}

	return Result{
		InstanceID: entry.InstanceID,
		Stdout:     output,
		ExitCode:   0,
	}, nil
}

func isKnownKind(kind string) bool {
	switch kind {
	case kindPsql, kindPgbench:
		return true
	default:
		return false
	}
}

func defaultCommand(kind string) string {
	switch kind {
	case kindPsql:
		return "psql"
	case kindPgbench:
		return "pgbench"
	default:
		return ""
	}
}

func buildExecArgs(kind string, command string, args []string) []string {
	execArgs := []string{command}
	switch kind {
	case kindPsql:
		execArgs = append(execArgs, args...)
		execArgs = append(execArgs, defaultDSN())
	case kindPgbench:
		execArgs = append(execArgs, "-h", "127.0.0.1", "-p", "5432", "-U", "sqlrs", "-d", "postgres")
		execArgs = append(execArgs, args...)
	default:
		execArgs = append(execArgs, args...)
	}
	return execArgs
}

func defaultDSN() string {
	return "postgres://sqlrs@127.0.0.1:5432/postgres"
}
