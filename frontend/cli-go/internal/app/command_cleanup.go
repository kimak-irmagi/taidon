package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sqlrs/cli/internal/cli"
)

var deleteInstanceDetailedFn = cli.DeleteInstanceDetailed
var startCleanupSpinnerFn = startCleanupSpinner

// cleanupPreparedInstance keeps composite prepare+run cleanup reporting in one place,
// matching the workflow described in docs/architecture/cli-maintainability-refactor.md.
func cleanupPreparedInstance(ctx context.Context, stderr io.Writer, runOpts cli.RunOptions, instanceID string, verbose bool) {
	if strings.TrimSpace(instanceID) == "" {
		return
	}
	stopSpinner := startCleanupSpinnerFn(instanceID, verbose)
	result, status, err := deleteInstanceDetailedFn(ctx, runOpts, instanceID)
	stopSpinner()
	if err != nil {
		if verbose {
			fmt.Fprintf(stderr, "cleanup failed for instance %s: %v\n", instanceID, err)
		} else {
			fmt.Fprintf(stderr, "cleanup failed: %v\n", err)
		}
		return
	}
	if status == http.StatusConflict || strings.EqualFold(result.Outcome, "blocked") {
		if verbose {
			fmt.Fprintf(stderr, "cleanup blocked for instance %s: %s\n", instanceID, formatCleanupResult(result))
		} else {
			fmt.Fprintf(stderr, "cleanup blocked for instance %s\n", instanceID)
		}
	}
}
