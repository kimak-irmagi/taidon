package alias

import (
	"fmt"
	"strings"

	"github.com/sqlrs/cli/internal/cli/runkind"
	"github.com/sqlrs/cli/internal/inputset"
	"gopkg.in/yaml.v3"
)

// LoadTarget loads and validates one execution-facing alias definition using
// the canonical alias-package schema rules.
func LoadTarget(target Target) (Definition, error) {
	return LoadTargetWithFS(target, inputset.OSFileSystem{})
}

// LoadTargetWithFS loads and validates one execution-facing alias definition
// against the supplied filesystem so ref-backed execution can reuse the same
// loader as live-host execution and inspection.
func LoadTargetWithFS(target Target, fs inputset.FileSystem) (Definition, error) {
	class := normalizeClass(target.Class)
	if class == "" {
		return Definition{}, fmt.Errorf("alias class is required")
	}
	switch class {
	case ClassPrepare:
		return loadPrepareAliasWithFS(target.Path, fs)
	case ClassRun:
		return loadRunAliasWithFS(target.Path, fs)
	default:
		return Definition{}, fmt.Errorf("alias class is required")
	}
}

func loadPrepareAlias(path string) (Definition, error) {
	return loadPrepareAliasWithFS(path, inputset.OSFileSystem{})
}

func loadPrepareAliasWithFS(path string, fs inputset.FileSystem) (Definition, error) {
	def, err := loadDefinition(path, fs)
	if err != nil {
		return Definition{}, err
	}
	def.Class = ClassPrepare
	switch def.Kind {
	case "":
		return Definition{}, userErrorf("prepare alias kind is required")
	case "psql", "lb":
	default:
		return Definition{}, userErrorf("unknown prepare alias kind: %s", def.Kind)
	}
	if len(def.Args) == 0 {
		return Definition{}, userErrorf("prepare alias args are required")
	}
	return def, nil
}

func loadRunAlias(path string) (Definition, error) {
	return loadRunAliasWithFS(path, inputset.OSFileSystem{})
}

func loadRunAliasWithFS(path string, fs inputset.FileSystem) (Definition, error) {
	def, err := loadDefinition(path, fs)
	if err != nil {
		return Definition{}, err
	}
	def.Class = ClassRun
	switch def.Kind {
	case "":
		return Definition{}, userErrorf("run alias kind is required")
	default:
		if !runkind.IsKnown(def.Kind) {
			return Definition{}, userErrorf("unknown run alias kind: %s", def.Kind)
		}
	}
	if strings.TrimSpace(def.Image) != "" {
		return Definition{}, userErrorf("run alias does not support image")
	}
	if len(def.Args) == 0 {
		return Definition{}, userErrorf("run alias args are required")
	}
	return def, nil
}

func loadDefinition(path string, fs inputset.FileSystem) (Definition, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	var payload struct {
		Kind  string   `yaml:"kind"`
		Image string   `yaml:"image"`
		Args  []string `yaml:"args"`
	}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		class := classifyPath(path)
		switch class {
		case ClassPrepare:
			return Definition{}, userErrorf("read prepare alias: %v", err)
		case ClassRun:
			return Definition{}, userErrorf("read run alias: %v", err)
		default:
			return Definition{}, err
		}
	}
	return Definition{
		Kind:  strings.ToLower(strings.TrimSpace(payload.Kind)),
		Image: strings.TrimSpace(payload.Image),
		Args:  payload.Args,
	}, nil
}
