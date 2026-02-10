package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"sqlrs/cli/internal/cli"
)

type configCommand struct {
	action    string
	path      string
	rawValue  string
	effective bool
}

func parseConfigArgs(args []string) (configCommand, bool, error) {
	var cmd configCommand
	if len(args) == 0 {
		return cmd, false, ExitErrorf(2, "Missing config command")
	}
	action := strings.TrimSpace(args[0])
	switch action {
	case "get":
		path := ""
		effective := false
		for _, arg := range args[1:] {
			switch strings.TrimSpace(arg) {
			case "--help", "-h":
				return cmd, true, nil
			case "--effective":
				effective = true
			default:
				if strings.HasPrefix(arg, "-") {
					return cmd, false, ExitErrorf(2, "Invalid arguments")
				}
				if path != "" {
					return cmd, false, ExitErrorf(2, "Too many arguments")
				}
				path = arg
			}
		}
		if path == "" {
			return cmd, false, ExitErrorf(2, "Missing config path")
		}
		cmd = configCommand{action: "get", path: path, effective: effective}
	case "set":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return cmd, true, nil
		}
		if len(args) < 3 {
			if len(args) == 2 {
				path, value, ok := splitConfigAssignment(args[1])
				if ok {
					cmd = configCommand{action: "set", path: path, rawValue: value}
					break
				}
			}
			return cmd, false, ExitErrorf(2, "Missing config path or value")
		}
		cmd = configCommand{action: "set", path: args[1], rawValue: args[2]}
	case "rm":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return cmd, true, nil
		}
		if len(args) < 2 {
			return cmd, false, ExitErrorf(2, "Missing config path")
		}
		cmd = configCommand{action: "rm", path: args[1]}
	case "schema":
		if len(args) >= 2 && (args[1] == "--help" || args[1] == "-h") {
			return cmd, true, nil
		}
		cmd = configCommand{action: "schema"}
	default:
		return cmd, false, ExitErrorf(2, "Unknown config command: %s", action)
	}
	return cmd, false, nil
}

func runConfig(stdout io.Writer, runOpts cli.ConfigOptions, args []string, output string) error {
	parsed, showHelp, err := parseConfigArgs(args)
	if err != nil {
		return err
	}
	if showHelp {
		cli.PrintConfigUsage(stdout)
		return nil
	}

	runOpts.Path = strings.TrimSpace(parsed.path)
	runOpts.Effective = parsed.effective

	switch parsed.action {
	case "get":
		value, err := cli.RunConfigGet(context.Background(), runOpts)
		if err != nil {
			return err
		}
		if output == "json" {
			return writeJSON(stdout, value)
		}
		fmt.Fprintf(stdout, "config path=%s\n", runOpts.Path)
		return writePrettyJSON(stdout, value)
	case "set":
		if strings.TrimSpace(parsed.rawValue) == "" {
			return ExitErrorf(2, "Missing config value")
		}
		value, err := parseJSONValue(parsed.rawValue)
		if err != nil {
			return err
		}
		runOpts.Value = value
		resp, err := cli.RunConfigSet(context.Background(), runOpts)
		if err != nil {
			return err
		}
		if output == "json" {
			return writeJSON(stdout, resp)
		}
		fmt.Fprintf(stdout, "config path=%s\n", runOpts.Path)
		return writePrettyJSON(stdout, resp)
	case "rm":
		resp, err := cli.RunConfigRemove(context.Background(), runOpts)
		if err != nil {
			return err
		}
		if output == "json" {
			return writeJSON(stdout, resp)
		}
		fmt.Fprintf(stdout, "config path=%s\n", runOpts.Path)
		return writePrettyJSON(stdout, resp)
	case "schema":
		value, err := cli.RunConfigSchema(context.Background(), runOpts)
		if err != nil {
			return err
		}
		if output == "json" {
			return writeJSON(stdout, value)
		}
		fmt.Fprintln(stdout, "config schema")
		return writePrettyJSON(stdout, value)
	default:
		return ExitErrorf(2, "Unknown config command: %s", parsed.action)
	}
}

func parseJSONValue(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		if auto, ok := autoQuoteJSONValue(raw); ok {
			return auto, nil
		}
		return nil, ExitErrorf(2, "Invalid JSON value: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, ExitErrorf(2, "Invalid JSON value: trailing data")
	}
	return value, nil
}

func splitConfigAssignment(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	idx := strings.Index(raw, "=")
	if idx <= 0 || idx >= len(raw)-1 {
		return "", "", false
	}
	path := strings.TrimSpace(raw[:idx])
	value := strings.TrimSpace(raw[idx+1:])
	if path == "" || value == "" {
		return "", "", false
	}
	return path, value, true
}

func autoQuoteJSONValue(raw string) (any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	lower := strings.ToLower(raw)
	switch lower {
	case "true", "false", "null":
		return nil, false
	}
	if looksLikeJSONValue(raw) {
		return nil, false
	}
	if !isBareword(raw) {
		return nil, false
	}
	return raw, true
}

func looksLikeJSONValue(raw string) bool {
	switch raw[0] {
	case '{', '[', '"':
		return true
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}

func isBareword(raw string) bool {
	for _, ch := range raw {
		if ch == '.' || ch == '-' || ch == '_' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		return false
	}
	return true
}

func writePrettyJSON(w io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}
