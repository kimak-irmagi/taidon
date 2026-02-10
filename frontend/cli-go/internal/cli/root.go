package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

var ErrHelp = errors.New("help requested")

type GlobalOptions struct {
	Profile   string
	Endpoint  string
	Mode      string
	Workspace string
	Output    string
	Timeout   time.Duration
	Verbose   bool
}

type Command struct {
	Name string
	Args []string
}

func ParseArgs(args []string) (GlobalOptions, []Command, error) {
	var opts GlobalOptions

	fs := flag.NewFlagSet("sqlrs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	profile := fs.String("profile", "", "config profile")
	endpoint := fs.String("endpoint", "", "override endpoint")
	mode := fs.String("mode", "", "override mode")
	workspace := fs.String("workspace", "", "workspace root")
	output := fs.String("output", "", "output format (human|json)")
	timeout := fs.String("timeout", "", "request timeout (e.g. 30s)")
	verbose := fs.Bool("verbose", false, "verbose logging")
	verboseShort := fs.Bool("v", false, "verbose logging")
	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return opts, nil, err
	}

	if *help || *helpShort {
		return opts, nil, ErrHelp
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return opts, nil, errors.New("missing command")
	}

	opts.Profile = *profile
	opts.Endpoint = *endpoint
	opts.Mode = *mode
	opts.Workspace = *workspace
	opts.Output = *output
	opts.Verbose = *verbose || *verboseShort

	if *timeout != "" {
		parsed, err := time.ParseDuration(*timeout)
		if err != nil {
			return opts, nil, fmt.Errorf("invalid timeout: %w", err)
		}
		opts.Timeout = parsed
	}

	commands, err := splitCommands(remaining)
	if err != nil {
		return opts, nil, err
	}
	return opts, commands, nil
}

func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sqlrs [global flags] <command> [command flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init     Initialize a workspace")
	fmt.Fprintln(w, "  ls       List names, instances, or states")
	fmt.Fprintln(w, "  rm       Remove an instance or state")
	fmt.Fprintln(w, "  run:psql  Run a command against an instance (psql)")
	fmt.Fprintln(w, "  run:pgbench  Run a command against an instance (pgbench)")
	fmt.Fprintln(w, "  plan:psql  Compute a prepare plan with psql")
	fmt.Fprintln(w, "  plan:lb    Compute a prepare plan with Liquibase")
	fmt.Fprintln(w, "  prepare:psql  Prepare a database state with psql")
	fmt.Fprintln(w, "  prepare:lb    Prepare a database state with Liquibase")
	fmt.Fprintln(w, "  status   Check service health")
	fmt.Fprintln(w, "  config   Manage server config")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Global flags:")
	fmt.Fprintln(w, "  --profile <name>        Config profile")
	fmt.Fprintln(w, "  --endpoint <url|auto>   Override endpoint")
	fmt.Fprintln(w, "  --mode <local|remote>   Override mode")
	fmt.Fprintln(w, "  --workspace <path>      Workspace root")
	fmt.Fprintln(w, "  --output <human|json>   Output format")
	fmt.Fprintln(w, "  --timeout <duration>   Request timeout (e.g. 30s)")
	fmt.Fprintln(w, "  -v, --verbose           Verbose logging")
}

func splitCommands(args []string) ([]Command, error) {
	if len(args) == 0 {
		return nil, errors.New("missing command")
	}
	if isCompositePrepareRun(args) {
		runIdx := findRunIndex(args)
		if runIdx > 0 {
			return []Command{
				{Name: args[0], Args: args[1:runIdx]},
				{Name: args[runIdx], Args: args[runIdx+1:]},
			}, nil
		}
	}
	return []Command{{Name: args[0], Args: args[1:]}}, nil
}

func isCompositePrepareRun(args []string) bool {
	if len(args) < 2 {
		return false
	}
	if !strings.HasPrefix(args[0], "prepare:") {
		return false
	}
	return findRunIndex(args) > 0
}

func findRunIndex(args []string) int {
	for i := 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "run:") {
			return i
		}
	}
	return -1
}

func isCommandToken(value string) bool {
	switch value {
	case "init", "ls", "rm", "plan", "prepare", "run", "status", "config":
		return true
	}
	if strings.HasPrefix(value, "prepare:") {
		return true
	}
	if strings.HasPrefix(value, "plan:") {
		return true
	}
	if strings.HasPrefix(value, "run:") {
		return true
	}
	return false
}
