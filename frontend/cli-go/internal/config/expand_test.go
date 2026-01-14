package config

import (
	"os"
	"testing"
)

func TestExpandMapUsesVarsAndEnv(t *testing.T) {
	if err := os.Setenv("SQLRS_TEST_ENV", "value"); err != nil {
		t.Fatalf("set env: %v", err)
	}
	defer os.Unsetenv("SQLRS_TEST_ENV")

	data := map[string]any{
		"path": "${SQLRSROOT}/run",
		"env":  "${SQLRS_TEST_ENV}",
		"nested": map[string]any{
			"state": "${StateDir}/x",
		},
		"list": []any{"${StateDir}/y"},
	}
	vars := map[string]string{
		"SQLRSROOT": "/root",
		"StateDir":  "/state",
	}

	ExpandMap(data, vars)

	if data["path"].(string) != "/root/run" {
		t.Fatalf("expected SQLRSROOT expansion")
	}
	if data["env"].(string) != "value" {
		t.Fatalf("expected env expansion")
	}
	nested := data["nested"].(map[string]any)
	if nested["state"].(string) != "/state/x" {
		t.Fatalf("expected StateDir expansion")
	}
	list := data["list"].([]any)
	if list[0].(string) != "/state/y" {
		t.Fatalf("expected list expansion")
	}
}
