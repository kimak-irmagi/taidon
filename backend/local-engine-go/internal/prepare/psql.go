package prepare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type psqlPrepared struct {
	normalizedArgs []string
	argsNormalized string
	inputHashes    []inputHash
	filePaths      []string
}

type inputHash struct {
	Kind  string
	Value string
}

func preparePsqlArgs(args []string, stdin *string) (psqlPrepared, error) {
	normalized := append([]string{}, args...)
	var inputHashes []inputHash
	var filePaths []string
	hasNoPsqlrc := false
	hasOnErrorStop := false
	usesStdin := false

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

		if handled, err := handleFileFlag(args, &i, stdin, &usesStdin, &inputHashes, &filePaths); err != nil {
			return psqlPrepared{}, err
		} else if handled {
			continue
		}

		if handled, err := handleCommandFlag(args, &i, &inputHashes); err != nil {
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

	if !hasNoPsqlrc {
		normalized = append(normalized, "-X")
	}
	if !hasOnErrorStop {
		normalized = append(normalized, "-v", "ON_ERROR_STOP=1")
	}

	return psqlPrepared{
		normalizedArgs: normalized,
		argsNormalized: strings.Join(normalized, " "),
		inputHashes:    inputHashes,
		filePaths:      filePaths,
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

func handleFileFlag(args []string, index *int, stdin *string, usesStdin *bool, inputHashes *[]inputHash, filePaths *[]string) (bool, error) {
	arg := args[*index]
	switch {
	case arg == "-f" || arg == "--file":
		if *index+1 >= len(args) {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for file flag", Details: arg}
		}
		path := args[*index+1]
		if err := addFileHash(path, stdin, usesStdin, inputHashes, filePaths); err != nil {
			return true, err
		}
		*index++
		return true, nil
	case strings.HasPrefix(arg, "--file="):
		path := strings.TrimPrefix(arg, "--file=")
		if path == "" {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for file flag", Details: arg}
		}
		if err := addFileHash(path, stdin, usesStdin, inputHashes, filePaths); err != nil {
			return true, err
		}
		return true, nil
	case strings.HasPrefix(arg, "-f") && len(arg) > 2:
		path := arg[2:]
		if err := addFileHash(path, stdin, usesStdin, inputHashes, filePaths); err != nil {
			return true, err
		}
		return true, nil
	default:
		return false, nil
	}
}

func handleCommandFlag(args []string, index *int, inputHashes *[]inputHash) (bool, error) {
	arg := args[*index]
	switch {
	case arg == "-c" || arg == "--command":
		if *index+1 >= len(args) {
			return true, ValidationError{Code: "invalid_argument", Message: "missing value for command flag", Details: arg}
		}
		cmd := args[*index+1]
		*inputHashes = append(*inputHashes, inputHash{
			Kind:  "command",
			Value: hashContent(cmd),
		})
		*index++
		return true, nil
	case strings.HasPrefix(arg, "--command="):
		cmd := strings.TrimPrefix(arg, "--command=")
		*inputHashes = append(*inputHashes, inputHash{
			Kind:  "command",
			Value: hashContent(cmd),
		})
		return true, nil
	case strings.HasPrefix(arg, "-c") && len(arg) > 2:
		cmd := arg[2:]
		*inputHashes = append(*inputHashes, inputHash{
			Kind:  "command",
			Value: hashContent(cmd),
		})
		return true, nil
	default:
		return false, nil
	}
}

func addFileHash(path string, stdin *string, usesStdin *bool, inputHashes *[]inputHash, filePaths *[]string) error {
	if path == "-" {
		*usesStdin = true
		if stdin == nil {
			return ValidationError{Code: "invalid_argument", Message: "stdin is required when using -f -"}
		}
		*inputHashes = append(*inputHashes, inputHash{
			Kind:  "stdin",
			Value: hashContent(*stdin),
		})
		return nil
	}
	if path == "" {
		return ValidationError{Code: "invalid_argument", Message: "file path is empty"}
	}
	if !filepath.IsAbs(path) {
		return ValidationError{Code: "invalid_argument", Message: "file path must be absolute", Details: path}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationError{Code: "invalid_argument", Message: "cannot read file", Details: fmt.Sprintf("%s: %v", path, err)}
	}
	*inputHashes = append(*inputHashes, inputHash{
		Kind:  "file",
		Value: hashContentBytes(data),
	})
	if filePaths != nil {
		*filePaths = append(*filePaths, path)
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

func hashContent(value string) string {
	return hashContentBytes([]byte(value))
}

func hashContentBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
