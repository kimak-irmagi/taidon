package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
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

func ParseArgs(args []string) (GlobalOptions, Command, error) {
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
		return opts, Command{}, err
	}

	if *help || *helpShort {
		return opts, Command{}, ErrHelp
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return opts, Command{}, errors.New("missing command")
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
			return opts, Command{}, fmt.Errorf("invalid timeout: %w", err)
		}
		opts.Timeout = parsed
	}

	return opts, Command{Name: remaining[0], Args: remaining[1:]}, nil
}

func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  sqlrs [global flags] <command> [command flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init     Initialize a workspace")
	fmt.Fprintln(w, "  ls       List names, instances, or states")
	fmt.Fprintln(w, "  rm       Remove an instance or state")
	fmt.Fprintln(w, "  prepare:psql  Prepare a database state with psql")
	fmt.Fprintln(w, "  status   Check service health")
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
