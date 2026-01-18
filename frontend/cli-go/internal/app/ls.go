package app

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"

	"sqlrs/cli/internal/cli"
)

type lsOptions struct {
	IncludeNames     bool
	IncludeInstances bool
	IncludeStates    bool
	IncludeJobs      bool
	IncludeTasks     bool

	FilterName     string
	FilterInstance string
	FilterState    string
	FilterJob      string
	FilterKind     string
	FilterImage    string

	Quiet    bool
	NoHeader bool
	LongIDs  bool
}

func parseLsFlags(args []string) (lsOptions, bool, error) {
	var opts lsOptions

	fs := flag.NewFlagSet("sqlrs ls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	names := fs.Bool("names", false, "list names")
	namesShort := fs.Bool("n", false, "list names")
	instances := fs.Bool("instances", false, "list instances")
	instancesShort := fs.Bool("i", false, "list instances")
	states := fs.Bool("states", false, "list states")
	statesShort := fs.Bool("s", false, "list states")
	jobs := fs.Bool("jobs", false, "list jobs")
	jobsShort := fs.Bool("j", false, "list jobs")
	tasks := fs.Bool("tasks", false, "list tasks")
	tasksShort := fs.Bool("t", false, "list tasks")
	all := fs.Bool("all", false, "list all object types")

	quiet := fs.Bool("quiet", false, "suppress headers and explanatory text")
	noHeader := fs.Bool("no-header", false, "do not print table header")
	longIDs := fs.Bool("long", false, "show full ids")

	name := fs.String("name", "", "filter by name")
	instance := fs.String("instance", "", "filter by instance id")
	state := fs.String("state", "", "filter by state id")
	job := fs.String("job", "", "filter by job id")
	kind := fs.String("kind", "", "filter by prepare kind")
	image := fs.String("image", "", "filter by base image")

	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		return opts, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}

	if *help || *helpShort {
		return opts, true, nil
	}

	if fs.NArg() > 0 {
		return opts, false, ExitErrorf(2, "Invalid arguments")
	}

	opts.IncludeNames = *names || *namesShort
	opts.IncludeInstances = *instances || *instancesShort
	opts.IncludeStates = *states || *statesShort
	opts.IncludeJobs = *jobs || *jobsShort
	opts.IncludeTasks = *tasks || *tasksShort
	if *all {
		opts.IncludeNames = true
		opts.IncludeInstances = true
		opts.IncludeStates = true
		opts.IncludeJobs = true
		opts.IncludeTasks = true
	}
	if !opts.IncludeNames && !opts.IncludeInstances && !opts.IncludeStates && !opts.IncludeJobs && !opts.IncludeTasks {
		opts.IncludeNames = true
		opts.IncludeInstances = true
	}

	opts.FilterName = strings.TrimSpace(*name)
	opts.FilterInstance = strings.TrimSpace(*instance)
	opts.FilterState = strings.TrimSpace(*state)
	opts.FilterJob = strings.TrimSpace(*job)
	opts.FilterKind = strings.TrimSpace(*kind)
	opts.FilterImage = strings.TrimSpace(*image)
	opts.Quiet = *quiet
	opts.NoHeader = *noHeader
	opts.LongIDs = *longIDs
	return opts, false, nil
}

func runLs(w io.Writer, runOpts cli.LsOptions, args []string, output string) error {
	opts, showHelp, err := parseLsFlags(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintLsUsage(w)
		return nil
	}

	runOpts.IncludeNames = opts.IncludeNames
	runOpts.IncludeInstances = opts.IncludeInstances
	runOpts.IncludeStates = opts.IncludeStates
	runOpts.IncludeJobs = opts.IncludeJobs
	runOpts.IncludeTasks = opts.IncludeTasks
	runOpts.FilterName = opts.FilterName
	runOpts.FilterInstance = opts.FilterInstance
	runOpts.FilterState = opts.FilterState
	runOpts.FilterJob = opts.FilterJob
	runOpts.FilterKind = opts.FilterKind
	runOpts.FilterImage = opts.FilterImage
	runOpts.Quiet = opts.Quiet
	runOpts.NoHeader = opts.NoHeader
	runOpts.Long = opts.LongIDs

	result, err := cli.RunLs(context.Background(), runOpts)
	if err != nil {
		var prefixErr *cli.IDPrefixError
		if errors.As(err, &prefixErr) {
			return ExitErrorf(2, prefixErr.Error())
		}
		var ambiguousErr *cli.AmbiguousPrefixError
		if errors.As(err, &ambiguousErr) {
			return ExitErrorf(2, ambiguousErr.Error())
		}
		return err
	}

	if output == "json" {
		return writeJSON(w, result)
	}

	cli.PrintLs(w, result, cli.LsPrintOptions{
		Quiet:    opts.Quiet,
		NoHeader: opts.NoHeader,
		LongIDs:  opts.LongIDs,
	})
	return nil
}
