package prepare

import (
	"os"
	"path/filepath"
	"strings"
)

type psqlPrepared struct {
	normalizedArgs []string
	argsNormalized string
	inputs         []psqlInput
	filePaths      []string
	workDir        string
}

func preparePsqlArgs(args []string, stdin *string) (psqlPrepared, error) {
	normalized := append([]string{}, args...)
	var inputs []psqlInput
	var filePaths []string
	hasNoPsqlrc := false
	hasOnErrorStop := false
	usesStdin := false
	workDir := ""
	cwd, _ := os.Getwd()

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "" {
			continue
		}

		if arg == "--" {
			if i+1 < len(args) {
				return psqlPrepared{}, ValidationError{Code: "invalid_argument", Message: "positional arguments are not allowed"}
			}
			continue
		}

		if isConnectionFlag(arg) {
			return psqlPrepared{}, ValidationError{Code: "invalid_argument", Message: "connection flags are not allowed", Details: arg}
		}

		if arg == "-X" || arg == "--no-psqlrc" {
			hasNoPsqlrc = true
			continue
		}

		if handled, err := handleVarFlag(args, &i, &hasOnErrorStop); err != nil {
			return psqlPrepared{}, err
		} else if handled {
			continue
		}

		if handled, err := handleFileFlag(args, &i, stdin, &usesStdin, &inputs, &filePaths, &workDir); err != nil {
			return psqlPrepared{}, err
		} else if handled {
			continue
		}

		if handled, err := handleCommandFlag(args, &i, &inputs); err != nil {
			return psqlPrepared{}, err
		} else if handled {
			continue
		}

		if strings.HasPrefix(arg, "-") {
			continue
		}
		return psqlPrepared{}, ValidationError{Code: "invalid_argument", Message: "positional database arguments are not allowed", Details: arg}
	}

	if usesStdin && stdin == nil {
		return psqlPrepared{}, ValidationError{Code: "invalid_argument", Message: "stdin is required when using -f -"}
	}
	if !usesStdin && stdin != nil {
		return psqlPrepared{}, ValidationError{Code: "invalid_argument", Message: "stdin is only valid with -f -"}
	}
	if workDir == "" && strings.TrimSpace(cwd) != "" {
		workDir = cwd
	}

	if !hasNoPsqlrc {
		normalized = append(normalized, "-X")
	}
	if !hasOnErrorStop {
		normalized = append(normalized, "-v", "ON_ERROR_STOP=1")
	}

	return psqlPrepared{
		normalizedArgs: normalized,
		argsNormalized: strings.Join(normalized, " "),
		inputs:         inputs,
		filePaths:      filePaths,
		workDir:        workDir,
	}, nil
}

func isConnectionFlag(arg string) bool {
	switch arg {
	case "-h", "-p", "-U", "-d", "--host", "--port", "--username", "--dbname", "--database":
		return true
	}
	if strings.HasPrefix(arg, "--host=") ||
		strings.HasPrefix(arg, "--port=") ||
		strings.HasPrefix(arg, "--username=") ||
		strings.HasPrefix(arg, "--dbname=") ||
		strings.HasPrefix(arg, "--database=") {
		return true
	}
	if strings.HasPrefix(arg, "-h") && len(arg) > 2 {
		return true
	}
	if strings.HasPrefix(arg, "-p") && len(arg) > 2 {
		return true
	}
	if strings.HasPrefix(arg, "-U") && len(arg) > 2 {
		return true
	}
	if strings.HasPrefix(arg, "-d") && len(arg) > 2 {
		return true
	}
	return false
}

func handleVarFlag(args []string, index *int, hasOnErrorStop *bool) (bool, error) {
	arg := args[*index]

	switch {
	case arg == "-v" || arg == "--set" || arg == "--variable":
		if *index+1 >= len(args) {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for variable flag", Details: arg}
		}
		val := args[*index+1]
		if err := checkOnErrorStop(val, hasOnErrorStop); err != nil {
			return true, err
		}
		*index++
		return true, nil
	case strings.HasPrefix(arg, "-v") && len(arg) > 2:
		val := arg[2:]
		if err := checkOnErrorStop(val, hasOnErrorStop); err != nil {
			return true, err
		}
		return true, nil
	case strings.HasPrefix(arg, "--set="):
		val := strings.TrimPrefix(arg, "--set=")
		if err := checkOnErrorStop(val, hasOnErrorStop); err != nil {
			return true, err
		}
		return true, nil
	case strings.HasPrefix(arg, "--variable="):
		val := strings.TrimPrefix(arg, "--variable=")
		if err := checkOnErrorStop(val, hasOnErrorStop); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func handleFileFlag(args []string, index *int, stdin *string, usesStdin *bool, inputs *[]psqlInput, filePaths *[]string, workDir *string) (bool, error) {
	arg := args[*index]
	switch {
	case arg == "-f" || arg == "--file":
		if *index+1 >= len(args) {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for file flag", Details: arg}
		}
		path := args[*index+1]
		if err := addFileInput(path, stdin, usesStdin, inputs, filePaths, workDir); err != nil {
			return true, err
		}
		*index++
		return true, nil
	case strings.HasPrefix(arg, "--file="):
		path := strings.TrimPrefix(arg, "--file=")
		if path == "" {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for file flag", Details: arg}
		}
		if err := addFileInput(path, stdin, usesStdin, inputs, filePaths, workDir); err != nil {
			return true, err
		}
		return true, nil
	case strings.HasPrefix(arg, "-f") && len(arg) > 2:
		path := arg[2:]
		if err := addFileInput(path, stdin, usesStdin, inputs, filePaths, workDir); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func handleCommandFlag(args []string, index *int, inputs *[]psqlInput) (bool, error) {
	arg := args[*index]
	switch {
	case arg == "-c" || arg == "--command":
		if *index+1 >= len(args) {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for command flag", Details: arg}
		}
		cmd := args[*index+1]
		*inputs = append(*inputs, psqlInput{kind: "command", value: cmd})
		*index++
		return true, nil
	case strings.HasPrefix(arg, "--command="):
		cmd := strings.TrimPrefix(arg, "--command=")
		*inputs = append(*inputs, psqlInput{kind: "command", value: cmd})
		return true, nil
	case strings.HasPrefix(arg, "-c") && len(arg) > 2:
		cmd := arg[2:]
		*inputs = append(*inputs, psqlInput{kind: "command", value: cmd})
		return true, nil
	default:
		return false, nil
	}
}

func addFileInput(path string, stdin *string, usesStdin *bool, inputs *[]psqlInput, filePaths *[]string, workDir *string) error {
	if path == "-" {
		*usesStdin = true
		if stdin == nil {
			return ValidationError{Code: "invalid_argument", Message: "stdin is required when using -f -"}
		}
		*inputs = append(*inputs, psqlInput{kind: "stdin", value: *stdin})
		return nil
	}
	if path == "" {
		return ValidationError{Code: "invalid_argument", Message: "file path is empty"}
	}
	if !filepath.IsAbs(path) {
		return ValidationError{Code: "invalid_argument", Message: "file path must be absolute", Details: path}
	}
	if _, err := os.Stat(path); err != nil {
		return ValidationError{Code: "invalid_argument", Message: "cannot read file", Details: path}
	}
	*inputs = append(*inputs, psqlInput{kind: "file", value: path})
	if filePaths != nil {
		*filePaths = append(*filePaths, path)
	}
	if workDir != nil && *workDir == "" {
		*workDir = filepath.Dir(path)
	}
	return nil
}

func checkOnErrorStop(value string, hasOnErrorStop *bool) error {
	name, val, ok := splitAssignment(value)
	if !ok {
		if strings.EqualFold(strings.TrimSpace(value), "ON_ERROR_STOP") {
			return ValidationError{Code: "invalid_argument", Message: "ON_ERROR_STOP must be set to 1"}
		}
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(name), "ON_ERROR_STOP") {
		*hasOnErrorStop = true
		if strings.TrimSpace(val) != "1" {
			return ValidationError{Code: "invalid_argument", Message: "ON_ERROR_STOP must be set to 1", Details: value}
		}
	}
	return nil
}

func splitAssignment(value string) (string, string, bool) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
