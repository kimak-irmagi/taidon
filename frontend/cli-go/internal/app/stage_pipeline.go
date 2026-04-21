package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/refctx"
)

type stageMode string

const (
	stageModePrepare stageMode = "prepare"
	stageModePlan    stageMode = "plan"
)

type stageRunRequest struct {
	mode                    stageMode
	kind                    string
	parsed                  prepareArgs
	workspaceRoot           string
	cwd                     string
	ref                     *refctx.Context
	output                  string
	relativizeLiquibasePath bool
}

type stageRuntime struct {
	opts    cli.PrepareOptions
	watch   bool
	cleanup func() error
}

func buildStageRuntime(stderr io.Writer, runOpts cli.PrepareOptions, cfg config.LoadedConfig, req stageRunRequest) (stageRuntime, error) {
	if req.mode == stageModePlan && req.parsed.WatchSpecified {
		return stageRuntime{}, ExitErrorf(2, "plan does not support --watch/--no-watch")
	}
	if req.kind == "lb" && len(req.parsed.PsqlArgs) == 0 {
		return stageRuntime{}, ExitErrorf(2, "liquibase command is required")
	}

	imageID, source, err := resolvePrepareImage(req.parsed.Image, cfg)
	if err != nil {
		return stageRuntime{}, err
	}
	if imageID == "" {
		return stageRuntime{}, ExitErrorf(2, "Missing base image id (set --image or dbms.image)")
	}
	if runOpts.Verbose {
		fmt.Fprint(stderr, formatImageSource(imageID, source))
	}

	runtime := stageRuntime{
		opts:  runOpts,
		watch: req.parsed.Watch,
	}
	runtime.opts.ImageID = imageID
	runtime.opts.DisableControlPrompt = usesPrepareRef(req.parsed, req.ref)

	switch req.kind {
	case "psql":
		bound, err := bindPreparePsqlInputsFn(runOpts, req.workspaceRoot, req.cwd, req.parsed, req.ref, os.Stdin)
		if err != nil {
			return stageRuntime{}, err
		}
		runtime.cleanup = bound.cleanup
		runtime.opts.PsqlArgs = bound.PsqlArgs
		runtime.opts.Stdin = bound.Stdin
		runtime.opts.PrepareKind = "psql"
	case "lb":
		liquibaseExec, err := resolveLiquibaseExec(cfg)
		if err != nil {
			return stageRuntime{}, err
		}
		liquibaseExecMode, err := resolveLiquibaseExecMode(cfg)
		if err != nil {
			return stageRuntime{}, err
		}
		bound, err := bindPrepareLiquibaseInputsFn(runOpts, req.workspaceRoot, req.cwd, req.parsed, req.ref, liquibaseExec, liquibaseExecMode, req.relativizeLiquibasePath)
		if err != nil {
			return stageRuntime{}, err
		}
		runtime.cleanup = bound.cleanup
		runtime.opts.LiquibaseArgs = bound.LiquibaseArgs
		runtime.opts.LiquibaseExec = liquibaseExec
		runtime.opts.LiquibaseExecMode = liquibaseExecMode
		runtime.opts.LiquibaseEnv = resolveLiquibaseEnv()
		runtime.opts.WorkDir = bound.WorkDir
		runtime.opts.PrepareKind = "lb"
	default:
		switch req.mode {
		case stageModePlan:
			return stageRuntime{}, ExitErrorf(2, "unsupported plan kind: %s", req.kind)
		default:
			return stageRuntime{}, ExitErrorf(2, "unsupported prepare kind: %s", req.kind)
		}
	}

	if req.mode == stageModePlan {
		runtime.opts.PlanOnly = true
	}
	return runtime, nil
}

func executePlanStage(stdout io.Writer, runtime stageRuntime, output string) error {
	result, err := runPlanFn(context.Background(), runtime.opts)
	if err != nil {
		return finishPrepareCleanup(err, runtime.cleanup)
	}
	if output == "json" {
		return finishPrepareCleanup(writeJSON(stdout, result), runtime.cleanup)
	}
	return finishPrepareCleanup(cli.PrintPlan(stdout, result), runtime.cleanup)
}

func executePrepareStage(w stdoutAndErr, runtime stageRuntime) (result client.PrepareJobResult, handled bool, err error) {
	cleanupOnReturn := true
	defer func() {
		if cleanupOnReturn {
			err = finishPrepareCleanup(err, runtime.cleanup)
		}
	}()

	if !runtime.watch {
		accepted, err := submitPrepareFn(context.Background(), runtime.opts)
		if err != nil {
			return client.PrepareJobResult{}, false, err
		}
		printPrepareJobRefs(w.stdout, accepted)
		if runtime.opts.CompositeRun {
			printRunSkipped(w.stdout, "prepare_not_watched")
		}
		return client.PrepareJobResult{}, true, nil
	}

	result, err = runPrepareFn(context.Background(), runtime.opts)
	if err != nil {
		var detached *cli.PrepareDetachedError
		if errors.As(err, &detached) {
			accepted := client.PrepareJobAccepted{
				JobID:     detached.JobID,
				StatusURL: "/v1/prepare-jobs/" + detached.JobID,
				EventsURL: "/v1/prepare-jobs/" + detached.JobID + "/events",
			}
			printPrepareJobRefs(w.stdout, accepted)
			if runtime.opts.CompositeRun {
				printRunSkipped(w.stdout, "prepare_detached")
			}
			return client.PrepareJobResult{}, true, nil
		}
		return client.PrepareJobResult{}, false, err
	}
	return result, false, nil
}
