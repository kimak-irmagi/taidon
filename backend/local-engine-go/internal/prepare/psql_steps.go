package prepare

import (
	"fmt"
	"strconv"
	"strings"
)

type psqlStep struct {
	args   []string
	inputs []psqlInput
	stdin  *string
}

func buildPsqlSteps(args []string, stdin *string) ([]psqlStep, error) {
	shared := make([]string, 0, len(args))
	steps := make([]psqlStep, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-c" || arg == "--command":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for command flag: %s", arg)
			}
			cmd := args[i+1]
			steps = append(steps, psqlStep{
				args:   append(append([]string{}, shared...), "-c", cmd),
				inputs: []psqlInput{{kind: "command", value: cmd}},
			})
			i++
		case strings.HasPrefix(arg, "--command="):
			cmd := strings.TrimPrefix(arg, "--command=")
			steps = append(steps, psqlStep{
				args:   append(append([]string{}, shared...), "-c", cmd),
				inputs: []psqlInput{{kind: "command", value: cmd}},
			})
		case strings.HasPrefix(arg, "-c") && len(arg) > 2:
			cmd := arg[2:]
			steps = append(steps, psqlStep{
				args:   append(append([]string{}, shared...), "-c", cmd),
				inputs: []psqlInput{{kind: "command", value: cmd}},
			})
		case arg == "-f" || arg == "--file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for file flag: %s", arg)
			}
			step, err := buildPsqlFileStep(shared, args[i+1], stdin)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
			i++
		case strings.HasPrefix(arg, "--file="):
			value := strings.TrimPrefix(arg, "--file=")
			if value == "" {
				return nil, fmt.Errorf("missing value for file flag: %s", arg)
			}
			step, err := buildPsqlFileStep(shared, value, stdin)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value := arg[2:]
			step, err := buildPsqlFileStep(shared, value, stdin)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
		default:
			shared = append(shared, arg)
		}
	}

	if len(steps) == 0 {
		return []psqlStep{{args: append([]string{}, shared...)}}, nil
	}
	return steps, nil
}

func buildPsqlFileStep(shared []string, value string, stdin *string) (psqlStep, error) {
	if value == "-" {
		if stdin == nil {
			return psqlStep{}, fmt.Errorf("stdin is required when using -f -")
		}
		return psqlStep{
			args:   append(append([]string{}, shared...), "-f", "-"),
			inputs: []psqlInput{{kind: "stdin", value: *stdin}},
			stdin:  stdin,
		}, nil
	}
	if strings.TrimSpace(value) == "" {
		return psqlStep{}, fmt.Errorf("file path is empty")
	}
	return psqlStep{
		args:   append(append([]string{}, shared...), "-f", value),
		inputs: []psqlInput{{kind: "file", value: value}},
	}, nil
}

func psqlStepForTask(steps []psqlStep, taskID string) (psqlStep, error) {
	if len(steps) == 0 {
		return psqlStep{}, fmt.Errorf("psql steps are required")
	}
	index, err := executeTaskIndex(taskID)
	if err != nil {
		return psqlStep{}, err
	}
	if index < 0 || index >= len(steps) {
		return psqlStep{}, fmt.Errorf("psql task index out of range: %d", index)
	}
	return steps[index], nil
}

func psqlStepForPreparedTask(prepared preparedRequest, taskID string) (psqlStep, error) {
	if len(prepared.psqlSteps) == 0 {
		return psqlStep{
			args:   append([]string{}, prepared.normalizedArgs...),
			inputs: append([]psqlInput{}, prepared.psqlInputs...),
			stdin:  prepared.request.Stdin,
		}, nil
	}
	return psqlStepForTask(prepared.psqlSteps, taskID)
}

func executeTaskIndex(taskID string) (int, error) {
	if strings.TrimSpace(taskID) == "" {
		return 0, nil
	}
	if !strings.HasPrefix(taskID, "execute-") {
		return 0, fmt.Errorf("invalid execute task id: %s", taskID)
	}
	raw := strings.TrimPrefix(taskID, "execute-")
	if strings.TrimSpace(raw) == "" {
		return 0, fmt.Errorf("invalid execute task id: %s", taskID)
	}
	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid execute task id: %s", taskID)
	}
	return index, nil
}
