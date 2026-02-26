package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"sqlrs/cli/internal/cli"
)

type rmOptions struct {
	IDPrefix string
	Recurse  bool
	Force    bool
	DryRun   bool
}

func parseRmFlags(args []string) (rmOptions, bool, error) {
	var opts rmOptions
	if err := validateNoUnicodeDashFlags(args, 2); err != nil {
		return opts, false, err
	}

	fs := flag.NewFlagSet("sqlrs rm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	recurse := fs.Bool("recurse", false, "remove descendant states and instances")
	recurseShort := fs.Bool("r", false, "remove descendant states and instances")
	force := fs.Bool("force", false, "ignore active connections")
	forceShort := fs.Bool("f", false, "ignore active connections")
	dryRun := fs.Bool("dry-run", false, "show intended actions only")

	help := fs.Bool("help", false, "show help")
	helpShort := fs.Bool("h", false, "show help")

	flags, positionals := splitRmArgs(args)
	if err := fs.Parse(flags); err != nil {
		return opts, false, ExitErrorf(2, "Invalid arguments: %v", err)
	}

	if *help || *helpShort {
		return opts, true, nil
	}

	if len(positionals) == 0 {
		return opts, false, ExitErrorf(2, "Missing id prefix")
	}
	if len(positionals) > 1 {
		return opts, false, ExitErrorf(2, "Too many arguments")
	}

	prefix := strings.TrimSpace(positionals[0])
	if prefix == "" {
		return opts, false, ExitErrorf(2, "Missing id prefix")
	}

	opts.IDPrefix = prefix
	opts.Recurse = *recurse || *recurseShort
	opts.Force = *force || *forceShort
	opts.DryRun = *dryRun
	return opts, false, nil
}

func splitRmArgs(args []string) ([]string, []string) {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	inPositionals := false
	for _, arg := range args {
		if inPositionals {
			positionals = append(positionals, arg)
			continue
		}
		if arg == "--" {
			inPositionals = true
			continue
		}
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return flags, positionals
}

func runRm(w io.Writer, runOpts cli.RmOptions, args []string, output string) error {
	opts, showHelp, err := parseRmFlags(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintRmUsage(w)
		return nil
	}

	runOpts.IDPrefix = opts.IDPrefix
	runOpts.Recurse = opts.Recurse
	runOpts.Force = opts.Force
	runOpts.DryRun = opts.DryRun

	result, err := cli.RunRm(context.Background(), runOpts)
	if err != nil {
		var prefixErr *cli.IDPrefixError
		if errors.As(err, &prefixErr) {
			return ExitErrorf(2, prefixErr.Error())
		}
		var ambiguousErr *cli.AmbiguousPrefixError
		if errors.As(err, &ambiguousErr) {
			return ExitErrorf(2, ambiguousErr.Error())
		}
		var resourceErr *cli.AmbiguousResourceError
		if errors.As(err, &resourceErr) {
			return ExitErrorf(2, resourceErr.Error())
		}
		return ExitErrorf(3, "Internal error: %v", err)
	}

	if result.NoMatch {
		fmt.Fprintf(os.Stderr, "warning: no matching instance or state for prefix %s\n", opts.IDPrefix)
		return nil
	}
	if result.Delete == nil {
		return nil
	}

	if output == "json" {
		if err := writeJSON(w, result.Delete); err != nil {
			return err
		}
	} else {
		cli.PrintRm(w, *result.Delete)
	}

	if result.Delete.Outcome == "blocked" {
		return ExitErrorf(4, "Deletion blocked")
	}
	return nil
}
