package cli

import (
	"fmt"
	"io"

	"sqlrs/cli/internal/client"
)

func PrintPlan(w io.Writer, result PlanResult) error {
	finalState, err := finalStateID(result.Tasks)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Final state: %s\n", finalState)
	fmt.Fprintln(w, "Tasks:")
	for i, task := range result.Tasks {
		fmt.Fprintf(w, "  %d. %s\n", i+1, formatPlanTask(task))
	}
	return nil
}

func finalStateID(tasks []client.PlanTask) (string, error) {
	for i := len(tasks) - 1; i >= 0; i-- {
		task := tasks[i]
		switch task.Type {
		case "prepare_instance":
			if task.Input != nil && task.Input.Kind == "state" && task.Input.ID != "" {
				return task.Input.ID, nil
			}
		case "state_execute":
			if task.OutputStateID != "" {
				return task.OutputStateID, nil
			}
		}
	}
	return "", fmt.Errorf("plan has no final state")
}

func formatPlanTask(task client.PlanTask) string {
	switch task.Type {
	case "plan":
		if task.PlannerKind != "" {
			return fmt.Sprintf("plan (planner: %s)", task.PlannerKind)
		}
		return "plan"
	case "state_execute":
		return fmt.Sprintf("state_execute input=%s hash=%s output=%s cached=%s",
			formatTaskInput(task.Input),
			task.TaskHash,
			task.OutputStateID,
			formatCached(task.Cached),
		)
	case "prepare_instance":
		mode := task.InstanceMode
		if mode == "" {
			mode = "unknown"
		}
		return fmt.Sprintf("prepare_instance input=%s mode=%s",
			formatTaskInput(task.Input),
			mode,
		)
	default:
		return task.Type
	}
}

func formatTaskInput(input *client.TaskInput) string {
	if input == nil {
		return "unknown"
	}
	if input.Kind == "" {
		return input.ID
	}
	return fmt.Sprintf("%s:%s", input.Kind, input.ID)
}

func formatCached(value *bool) string {
	if value == nil {
		return "n/a"
	}
	if *value {
		return "yes"
	}
	return "no"
}
