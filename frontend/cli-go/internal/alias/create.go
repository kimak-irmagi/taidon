package alias

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sqlrs/cli/internal/cli/runkind"
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
	"gopkg.in/yaml.v3"
)

var createWritePayloadFn = func(w io.Writer, payload []byte) (int, error) {
	return w.Write(payload)
}

// CreateOptions carries the workspace-bounded inputs for materializing a new
// repo-tracked alias file, as described in docs/architecture/alias-create-flow.md.
type CreateOptions struct {
	WorkspaceRoot string
	CWD           string
	Ref           string
	Class         Class
	Kind          string
	Args          []string
}

// CreatePlan is the in-memory alias payload produced before the writer touches
// disk. It keeps the derived path and payload together for testing and rendering.
type CreatePlan struct {
	Target  Target
	Kind    string
	Image   string
	Args    []string
	Payload []byte
}

// CreateResult reports the created alias file to the app/cli layers.
type CreateResult struct {
	Type  Class  `json:"type"`
	Ref   string `json:"ref"`
	File  string `json:"file"`
	Path  string `json:"path"`
	Kind  string `json:"kind,omitempty"`
	Image string `json:"image,omitempty"`
}

// ResolveCreateTarget derives the workspace-bounded target path for a new alias
// without checking whether the file already exists.
func ResolveCreateTarget(opts CreateOptions) (Target, error) {
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return Target{}, fmt.Errorf("workspace root is required to create aliases")
	}
	workspaceRoot = filepath.Clean(workspaceRoot)

	cwd := strings.TrimSpace(opts.CWD)
	if cwd == "" {
		cwd = workspaceRoot
	}
	cwd = filepath.Clean(cwd)

	ref := strings.TrimSpace(opts.Ref)
	if ref == "" {
		return Target{}, fmt.Errorf("alias ref is required")
	}
	if strings.HasSuffix(strings.ToLower(ref), prepareSuffix) || strings.HasSuffix(strings.ToLower(ref), runSuffix) {
		return Target{}, fmt.Errorf("alias ref must be a logical stem")
	}
	if strings.HasSuffix(ref, ".") {
		return Target{}, fmt.Errorf("alias ref must be a logical stem")
	}

	class := normalizeClass(opts.Class)
	if class == "" {
		return Target{}, fmt.Errorf("alias class is required")
	}

	relative := filepath.FromSlash(ref)
	switch class {
	case ClassPrepare:
		relative += prepareSuffix
	case ClassRun:
		relative += runSuffix
	}

	path, err := resolvePathWithinWorkspace(relative, workspaceRoot, cwd)
	if err != nil {
		return Target{}, err
	}
	return Target{
		Class: class,
		Ref:   ref,
		File:  workspaceRelativePath(path, workspaceRoot),
		Path:  path,
	}, nil
}

// Create validates the wrapped command, derives the target alias file, and
// writes the repo-tracked payload without overwriting an existing file.
func Create(opts CreateOptions) (result CreateResult, err error) {
	target, err := ResolveCreateTarget(opts)
	if err != nil {
		return CreateResult{}, err
	}

	if _, statErr := os.Stat(target.Path); statErr == nil {
		return CreateResult{}, fmt.Errorf("alias file already exists: %s", target.Path)
	} else if !os.IsNotExist(statErr) {
		return CreateResult{}, statErr
	}

	controlArgs, payloadArgs, err := splitCreateArgs(target.Class, opts.Args)
	if err != nil {
		return CreateResult{}, err
	}
	if len(payloadArgs) == 0 {
		return CreateResult{}, fmt.Errorf("wrapped command args are required")
	}

	kind := strings.ToLower(strings.TrimSpace(opts.Kind))
	if err := validateCreateKind(target.Class, kind); err != nil {
		return CreateResult{}, err
	}

	image, err := parseCreateControlArgs(target.Class, controlArgs)
	if err != nil {
		return CreateResult{}, err
	}

	rewritten, err := rewriteCreateArgs(target.Class, kind, payloadArgs, opts.WorkspaceRoot, opts.CWD, filepath.Dir(target.Path))
	if err != nil {
		return CreateResult{}, err
	}

	plan := CreatePlan{
		Target: target,
		Kind:   kind,
		Image:  image,
		Args:   rewritten,
	}
	plan.Payload, err = renderCreatePayload(plan)
	if err != nil {
		return CreateResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(target.Path), 0o700); err != nil {
		return CreateResult{}, err
	}
	f, err := os.OpenFile(target.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return CreateResult{}, err
	}
	defer func() {
		closeErr := f.Close()
		if err != nil {
			if removeErr := os.Remove(target.Path); removeErr != nil && !os.IsNotExist(removeErr) {
				err = fmt.Errorf("%w (cleanup failed: %v)", err, removeErr)
			}
			return
		}
		if closeErr != nil {
			err = closeErr
		}
	}()

	if _, err = createWritePayloadFn(f, plan.Payload); err != nil {
		return CreateResult{}, err
	}

	result = CreateResult{
		Type:  target.Class,
		Ref:   target.Ref,
		File:  target.File,
		Path:  target.Path,
		Kind:  kind,
		Image: image,
	}
	return result, nil
}

func validateCreateKind(class Class, kind string) error {
	switch class {
	case ClassPrepare:
		switch kind {
		case "psql", "lb":
			return nil
		default:
			return fmt.Errorf("unknown prepare alias kind: %s", kind)
		}
	case ClassRun:
		if !runkind.IsKnown(kind) {
			return fmt.Errorf("unknown run alias kind: %s", kind)
		}
		return nil
	default:
		return fmt.Errorf("alias class is required")
	}
}

func renderCreatePayload(plan CreatePlan) ([]byte, error) {
	payload := struct {
		Kind  string   `yaml:"kind"`
		Image string   `yaml:"image,omitempty"`
		Args  []string `yaml:"args"`
	}{
		Kind:  plan.Kind,
		Image: strings.TrimSpace(plan.Image),
		Args:  plan.Args,
	}
	return yaml.Marshal(payload)
}

func rewriteCreateArgs(class Class, kind string, args []string, workspaceRoot string, cwd string, aliasDir string) ([]string, error) {
	switch class {
	case ClassPrepare:
		switch kind {
		case "psql":
			if issues := inputpsql.ValidateArgs(args, inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil), inputset.OSFileSystem{}); len(issues) > 0 {
				return nil, fmt.Errorf(issues[0].Message)
			}
			return relativizePsqlCreateArgs(args, workspaceRoot, cwd, aliasDir)
		case "lb":
			if issues := inputliquibase.ValidateArgs(args, inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil), inputset.OSFileSystem{}); len(issues) > 0 {
				return nil, fmt.Errorf(issues[0].Message)
			}
			return relativizeLiquibaseCreateArgs(args, workspaceRoot, cwd, aliasDir)
		default:
			return nil, fmt.Errorf("unknown prepare alias kind: %s", kind)
		}
	case ClassRun:
		switch kind {
		case runkind.KindPsql:
			if issues := inputpsql.ValidateArgs(args, inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil), inputset.OSFileSystem{}); len(issues) > 0 {
				return nil, fmt.Errorf(issues[0].Message)
			}
			return relativizePsqlCreateArgs(args, workspaceRoot, cwd, aliasDir)
		case runkind.KindPgbench:
			if issues := inputpgbench.ValidateArgs(args, inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil), inputset.OSFileSystem{}); len(issues) > 0 {
				return nil, fmt.Errorf(issues[0].Message)
			}
			return relativizePgbenchCreateArgs(args, workspaceRoot, cwd, aliasDir)
		default:
			return nil, fmt.Errorf("unknown run alias kind: %s", kind)
		}
	default:
		return nil, fmt.Errorf("alias class is required")
	}
}

func splitCreateArgs(class Class, args []string) ([]string, []string, error) {
	if len(args) == 0 {
		return nil, nil, nil
	}

	controlArgs := make([]string, 0, len(args))
	payloadStart := len(args)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			payloadStart = i + 1
			break
		}
		if !isCreateControlArg(class, arg) {
			payloadStart = i
			break
		}
		controlArgs = append(controlArgs, arg)
		if createControlArgTakesValue(arg) {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("Missing value for %s", arg)
			}
			controlArgs = append(controlArgs, args[i+1])
			i++
		}
	}

	payload := make([]string, 0, len(args)-payloadStart)
	if payloadStart < len(args) {
		payload = append(payload, args[payloadStart:]...)
	}
	if err := rejectCreateControlArgsInPayload(class, payload); err != nil {
		return nil, nil, err
	}
	return controlArgs, payload, nil
}

func parseCreateControlArgs(class Class, controlArgs []string) (string, error) {
	image := ""
	for i := 0; i < len(controlArgs); i++ {
		arg := controlArgs[i]
		switch class {
		case ClassPrepare:
			switch {
			case arg == "--image":
				if i+1 >= len(controlArgs) {
					return "", fmt.Errorf("Missing value for --image")
				}
				image = strings.TrimSpace(controlArgs[i+1])
				i++
			case strings.HasPrefix(arg, "--image="):
				image = strings.TrimSpace(strings.TrimPrefix(arg, "--image="))
			default:
				return "", fmt.Errorf("unknown prepare alias option: %s", arg)
			}
		case ClassRun:
			return "", fmt.Errorf("unknown run alias option: %s", arg)
		default:
			return "", fmt.Errorf("alias class is required")
		}
	}
	return strings.TrimSpace(image), nil
}

func isCreateControlArg(class Class, arg string) bool {
	switch class {
	case ClassPrepare:
		return arg == "--image" || strings.HasPrefix(arg, "--image=")
	default:
		return false
	}
}

func createControlArgTakesValue(arg string) bool {
	return arg == "--image"
}

func rejectCreateControlArgsInPayload(class Class, payload []string) error {
	for _, arg := range payload {
		switch class {
		case ClassPrepare:
			if arg == "--image" || strings.HasPrefix(arg, "--image=") ||
				arg == "--watch" || arg == "--no-watch" ||
				arg == "--instance" || strings.HasPrefix(arg, "--instance=") {
				return fmt.Errorf("wrapped command flags must appear before tool args")
			}
		case ClassRun:
			if arg == "--image" || strings.HasPrefix(arg, "--image=") ||
				arg == "--watch" || arg == "--no-watch" ||
				arg == "--instance" || strings.HasPrefix(arg, "--instance=") {
				return fmt.Errorf("wrapped command flags must appear before tool args")
			}
		}
	}
	return nil
}

func relativizePsqlCreateArgs(args []string, workspaceRoot string, cwd string, aliasDir string) ([]string, error) {
	rewritten := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			rewritten = append(rewritten, arg)
			if i+1 >= len(args) {
				continue
			}
			value, err := relativizeCreatePathArg(args[i+1], workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, value)
			i++
		case strings.HasPrefix(arg, "--file="):
			value, err := relativizeCreatePathArg(strings.TrimPrefix(arg, "--file="), workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "--file="+value)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value, err := relativizeCreatePathArg(arg[2:], workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "-f"+value)
		default:
			rewritten = append(rewritten, arg)
		}
	}
	return rewritten, nil
}

func relativizeLiquibaseCreateArgs(args []string, workspaceRoot string, cwd string, aliasDir string) ([]string, error) {
	rewritten := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--changelog-file" || arg == "--defaults-file":
			rewritten = append(rewritten, arg)
			if i+1 >= len(args) {
				continue
			}
			value, err := relativizeCreateLiquibaseValue(args[i+1], workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, value)
			i++
		case strings.HasPrefix(arg, "--changelog-file="):
			value, err := relativizeCreateLiquibaseValue(strings.TrimPrefix(arg, "--changelog-file="), workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "--changelog-file="+value)
		case strings.HasPrefix(arg, "--defaults-file="):
			value, err := relativizeCreateLiquibaseValue(strings.TrimPrefix(arg, "--defaults-file="), workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "--defaults-file="+value)
		case arg == "--searchPath" || arg == "--search-path":
			rewritten = append(rewritten, arg)
			if i+1 >= len(args) {
				continue
			}
			value, err := relativizeCreateLiquibaseSearchPath(args[i+1], workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, value)
			i++
		case strings.HasPrefix(arg, "--searchPath="):
			value, err := relativizeCreateLiquibaseSearchPath(strings.TrimPrefix(arg, "--searchPath="), workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "--searchPath="+value)
		case strings.HasPrefix(arg, "--search-path="):
			value, err := relativizeCreateLiquibaseSearchPath(strings.TrimPrefix(arg, "--search-path="), workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "--search-path="+value)
		default:
			rewritten = append(rewritten, arg)
		}
	}
	return rewritten, nil
}

func relativizePgbenchCreateArgs(args []string, workspaceRoot string, cwd string, aliasDir string) ([]string, error) {
	rewritten := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--file":
			rewritten = append(rewritten, arg)
			if i+1 >= len(args) {
				continue
			}
			value, err := relativizeCreatePgbenchValue(args[i+1], workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, value)
			i++
		case strings.HasPrefix(arg, "--file="):
			value, err := relativizeCreatePgbenchValue(strings.TrimPrefix(arg, "--file="), workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "--file="+value)
		case strings.HasPrefix(arg, "-f") && len(arg) > 2:
			value, err := relativizeCreatePgbenchValue(arg[2:], workspaceRoot, cwd, aliasDir)
			if err != nil {
				return nil, err
			}
			rewritten = append(rewritten, "-f"+value)
		default:
			rewritten = append(rewritten, arg)
		}
	}
	return rewritten, nil
}

func relativizeCreatePathArg(value string, workspaceRoot string, cwd string, aliasDir string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" || cleaned == "-" {
		return value, nil
	}
	resolved, err := inputset.NewWorkspaceResolver(workspaceRoot, cwd, nil).ResolvePath(cleaned)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(aliasDir, resolved)
	if err != nil {
		return filepath.ToSlash(resolved), nil
	}
	return filepath.ToSlash(rel), nil
}

func relativizeCreateLiquibaseValue(value string, workspaceRoot string, cwd string, aliasDir string) (string, error) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" || looksLikeLiquibaseRemoteRef(cleaned) {
		return value, nil
	}
	return relativizeCreatePathArg(cleaned, workspaceRoot, cwd, aliasDir)
}

func relativizeCreateLiquibaseSearchPath(value string, workspaceRoot string, cwd string, aliasDir string) (string, error) {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" || looksLikeLiquibaseRemoteRef(item) {
			out = append(out, item)
			continue
		}
		rel, err := relativizeCreatePathArg(item, workspaceRoot, cwd, aliasDir)
		if err != nil {
			return "", err
		}
		out = append(out, rel)
	}
	return strings.Join(out, ","), nil
}

func relativizeCreatePgbenchValue(value string, workspaceRoot string, cwd string, aliasDir string) (string, error) {
	path, weight := inputset.SplitPgbenchFileArgValue(value)
	cleaned := strings.TrimSpace(path)
	if cleaned == "" || cleaned == "-" {
		return value, nil
	}
	rel, err := relativizeCreatePathArg(cleaned, workspaceRoot, cwd, aliasDir)
	if err != nil {
		return "", err
	}
	return rel + weight, nil
}
