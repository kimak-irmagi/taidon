package discover

import (
	"os"
	"strings"

	"github.com/sqlrs/cli/internal/alias"
	"github.com/sqlrs/cli/internal/inputset"
	inputliquibase "github.com/sqlrs/cli/internal/inputset/liquibase"
	inputpgbench "github.com/sqlrs/cli/internal/inputset/pgbench"
	inputpsql "github.com/sqlrs/cli/internal/inputset/psql"
	"gopkg.in/yaml.v3"
)

func loadAliasCoverage(workspaceRoot string) (map[string]struct{}, error) {
	entries, err := alias.Scan(alias.ScanOptions{
		WorkspaceRoot: workspaceRoot,
		CWD:           workspaceRoot,
		From:          "workspace",
		Depth:         "recursive",
	})
	if err != nil {
		return nil, err
	}
	coverage := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		coverage[entry.File] = struct{}{}
		covered, err := aliasCoveragePaths(workspaceRoot, entry)
		if err != nil {
			continue
		}
		for path := range covered {
			for _, key := range discoverPathKeys(workspaceRoot, "", path) {
				coverage[key] = struct{}{}
			}
		}
	}
	return coverage, nil
}

type discoverAliasDefinition struct {
	Kind string   `yaml:"kind"`
	Args []string `yaml:"args"`
}

func aliasCoveragePaths(workspaceRoot string, entry alias.Entry) (map[string]struct{}, error) {
	if strings.TrimSpace(entry.Path) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(entry.Path)
	if err != nil {
		return nil, err
	}
	var def discoverAliasDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}

	def.Kind = strings.ToLower(strings.TrimSpace(def.Kind))
	if len(def.Args) == 0 {
		return nil, nil
	}

	resolver := inputset.NewAliasResolver(workspaceRoot, entry.Path)
	var inputSet inputset.InputSet
	switch entry.Class {
	case alias.ClassPrepare:
		switch def.Kind {
		case "psql":
			inputSet, err = inputpsql.Collect(def.Args, resolver, inputset.OSFileSystem{})
		case "lb":
			inputSet, err = inputliquibase.Collect(def.Args, resolver, inputset.OSFileSystem{})
		default:
			return nil, nil
		}
	case alias.ClassRun:
		switch def.Kind {
		case "psql":
			inputSet, err = inputpsql.Collect(def.Args, resolver, inputset.OSFileSystem{})
		case "pgbench":
			inputSet, err = inputpgbench.Collect(def.Args, resolver, inputset.OSFileSystem{})
		default:
			return nil, nil
		}
	default:
		return nil, nil
	}
	if err != nil {
		return nil, nil
	}

	covered := make(map[string]struct{}, len(inputSet.Entries))
	for _, entry := range inputSet.Entries {
		covered[entry.AbsPath] = struct{}{}
	}
	return covered, nil
}
