package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"sqlrs/cli/internal/cli"
	"sqlrs/cli/internal/client"
)

func parseWatchArgs(args []string) (string, bool, error) {
	if err := validateNoUnicodeDashFlags(args, 1); err != nil {
		return "", false, err
	}
	jobID := ""
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			return "", true, nil
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, ExitErrorf(2, "Unknown watch option: %s", arg)
			}
			if jobID != "" {
				return "", false, ExitErrorf(2, "watch accepts exactly one job id")
			}
			jobID = strings.TrimSpace(arg)
		}
	}
	if jobID == "" {
		return "", false, ExitErrorf(2, "Missing prepare job id")
	}
	return jobID, false, nil
}

func runWatch(stdout io.Writer, runOpts cli.PrepareOptions, args []string) error {
	jobID, showHelp, err := parseWatchArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintWatchUsage(stdout)
		return nil
	}
	status, err := cli.RunWatch(context.Background(), runOpts, jobID)
	if err != nil {
		var detached *cli.PrepareDetachedError
		if errors.As(err, &detached) {
			printPrepareJobRefs(stdout, prepareAcceptedFromDetached(detached.JobID))
			return nil
		}
		return err
	}
	if status.Status == "failed" {
		if status.Error != nil && status.Error.Message != "" {
			if status.Error.Details != "" {
				return fmt.Errorf("%s: %s", status.Error.Message, status.Error.Details)
			}
			return fmt.Errorf("%s", status.Error.Message)
		}
		return fmt.Errorf("prepare job failed")
	}
	return nil
}

func prepareAcceptedFromDetached(jobID string) client.PrepareJobAccepted {
	return client.PrepareJobAccepted{
		JobID:     jobID,
		StatusURL: "/v1/prepare-jobs/" + jobID,
		EventsURL: "/v1/prepare-jobs/" + jobID + "/events",
	}
}
