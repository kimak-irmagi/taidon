package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

type RmOptions struct {
	ProfileName    string
	Mode           string
	Endpoint       string
	Autostart      bool
	DaemonPath     string
	RunDir         string
	StateDir       string
	Timeout        time.Duration
	StartupTimeout time.Duration
	Verbose        bool

	IDPrefix string
	Recurse  bool
	Force    bool
	DryRun   bool
}

type RmResult struct {
	Delete  *client.DeleteResult
	NoMatch bool
}

type AmbiguousResourceError struct {
	Prefix string
}

func (e *AmbiguousResourceError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("ambiguous id prefix: %s matches both instance and state", e.Prefix)
}

func RunRm(ctx context.Context, opts RmOptions) (RmResult, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	endpoint := strings.TrimSpace(opts.Endpoint)
	authToken := ""

	if mode == "local" {
		if endpoint == "" {
			endpoint = "auto"
		}
		if endpoint == "auto" {
			if opts.Verbose {
				fmt.Fprintln(os.Stderr, "checking local engine state")
			}
			resolved, err := daemon.ConnectOrStart(ctx, daemon.ConnectOptions{
				Endpoint:       endpoint,
				Autostart:      opts.Autostart,
				DaemonPath:     opts.DaemonPath,
				RunDir:         opts.RunDir,
				StateDir:       opts.StateDir,
				StartupTimeout: opts.StartupTimeout,
				ClientTimeout:  opts.Timeout,
				Verbose:        opts.Verbose,
			})
			if err != nil {
				return RmResult{}, err
			}
			endpoint = resolved.Endpoint
			authToken = resolved.AuthToken
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "engine ready at %s\n", endpoint)
			}
		}
	} else if mode == "remote" {
		if endpoint == "" || endpoint == "auto" {
			return RmResult{}, fmt.Errorf("remote mode requires explicit endpoint")
		}
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "using remote endpoint %s\n", endpoint)
		}
	}

	normalized, err := normalizeIDPrefix("resource", opts.IDPrefix)
	if err != nil {
		return RmResult{}, err
	}

	cliClient := client.New(endpoint, client.Options{Timeout: opts.Timeout, AuthToken: authToken})
	instances, err := cliClient.ListInstances(ctx, client.ListFilters{IDPrefix: normalized})
	if err != nil {
		return RmResult{}, err
	}
	states, err := cliClient.ListStates(ctx, client.ListFilters{IDPrefix: normalized})
	if err != nil {
		return RmResult{}, err
	}

	if len(instances) > 1 {
		return RmResult{}, &AmbiguousPrefixError{Kind: "instance", Prefix: opts.IDPrefix}
	}
	if len(states) > 1 {
		return RmResult{}, &AmbiguousPrefixError{Kind: "state", Prefix: opts.IDPrefix}
	}
	if len(instances) == 1 && len(states) == 1 {
		return RmResult{}, &AmbiguousResourceError{Prefix: opts.IDPrefix}
	}
	if len(instances) == 0 && len(states) == 0 {
		return RmResult{NoMatch: true}, nil
	}

	deleteOpts := client.DeleteOptions{
		Recurse: opts.Recurse,
		Force:   opts.Force,
		DryRun:  opts.DryRun,
	}
	if len(instances) == 1 {
		result, _, err := cliClient.DeleteInstance(ctx, instances[0].InstanceID, deleteOpts)
		if err != nil {
			return RmResult{}, err
		}
		return RmResult{Delete: &result}, nil
	}

	result, _, err := cliClient.DeleteState(ctx, states[0].StateID, deleteOpts)
	if err != nil {
		return RmResult{}, err
	}
	return RmResult{Delete: &result}, nil
}

func PrintRm(w io.Writer, result client.DeleteResult) {
	printRootNode(w, result)
}

func printRootNode(w io.Writer, result client.DeleteResult) {
	if strings.TrimSpace(result.Root.ID) == "" {
		return
	}
	printNodeLine(w, result.Root, "", true, result)
	printNodeChildren(w, result.Root.Children, "", result)
}

func printNodeChildren(w io.Writer, children []client.DeleteNode, prefix string, result client.DeleteResult) {
	for i, child := range children {
		last := i == len(children)-1
		linePrefix := prefix
		if last {
			linePrefix += "`-- "
		} else {
			linePrefix += "|-- "
		}
		printNodeLine(w, child, linePrefix, false, result)

		nextPrefix := prefix
		if last {
			nextPrefix += "    "
		} else {
			nextPrefix += "|   "
		}
		if len(child.Children) > 0 {
			printNodeChildren(w, child.Children, nextPrefix, result)
		}
	}
}

func printNodeLine(w io.Writer, node client.DeleteNode, prefix string, isRoot bool, result client.DeleteResult) {
	action := nodeAction(node, result)
	label := fmt.Sprintf("%s %s %s", node.Kind, strings.ToLower(node.ID), action)
	if node.Kind == "instance" {
		label = fmt.Sprintf("%s (connections=%d)", label, nodeConnections(node))
	}
	if isRoot {
		fmt.Fprintln(w, label)
		return
	}
	fmt.Fprintln(w, prefix+label)
}

func nodeAction(node client.DeleteNode, result client.DeleteResult) string {
	if node.Blocked != "" {
		return fmt.Sprintf("blocked (%s)", node.Blocked)
	}
	if result.DryRun || result.Outcome == "blocked" {
		return "would delete"
	}
	return "deleted"
}

func nodeConnections(node client.DeleteNode) int {
	if node.Connections == nil {
		return 0
	}
	return *node.Connections
}
