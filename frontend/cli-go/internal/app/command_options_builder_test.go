package app

import (
	"testing"
	"time"

	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/paths"
)

func TestCommandContextBuilders(t *testing.T) {
	ctx := commandContext{
		cfgResult: config.LoadedConfig{
			Paths: paths.Dirs{StateDir: "/state"},
		},
		profileName:          "remote",
		profile:              config.ProfileConfig{Endpoint: "http://engine.example", Autostart: true},
		mode:                 "remote",
		authToken:            "secret",
		daemonPath:           "/bin/sqlrs-engine",
		runDir:               "/run/sqlrs",
		engineRunDir:         "/wsl/run",
		engineStatePath:      "/wsl/engine.json",
		engineStoreDir:       "/wsl/store",
		engineHostStorePath:  "/host/store",
		engineWSLMountUnit:   "sqlrs.mount",
		engineWSLMountFSType: "btrfs",
		wslDistro:            "Ubuntu",
		timeout:              30 * time.Second,
		idleTimeout:          90 * time.Second,
		startupTimeout:       7 * time.Second,
		verbose:              true,
	}

	prepareOpts := ctx.prepareOptions(true)
	if !prepareOpts.CompositeRun {
		t.Fatalf("expected CompositeRun=true")
	}
	if prepareOpts.ProfileName != "remote" || prepareOpts.Endpoint != "http://engine.example" {
		t.Fatalf("unexpected prepare options: %+v", prepareOpts)
	}
	if prepareOpts.WSLVHDXPath != "/host/store" || prepareOpts.WSLMountUnit != "sqlrs.mount" {
		t.Fatalf("unexpected prepare WSL options: %+v", prepareOpts)
	}

	runOpts := ctx.runOptions()
	if runOpts.ProfileName != "remote" || runOpts.AuthToken != "secret" {
		t.Fatalf("unexpected run options: %+v", runOpts)
	}
	if runOpts.EngineStoreDir != "/wsl/store" || runOpts.WSLDistro != "Ubuntu" {
		t.Fatalf("unexpected run engine options: %+v", runOpts)
	}

	statusOpts := ctx.statusOptions()
	if statusOpts.RunDir != "/run/sqlrs" || statusOpts.StateDir != "/state" {
		t.Fatalf("unexpected status options: %+v", statusOpts)
	}

	configOpts := ctx.configOptions()
	if configOpts.Timeout != 30*time.Second || !configOpts.Verbose {
		t.Fatalf("unexpected config options: %+v", configOpts)
	}

	lsOpts := ctx.lsOptions()
	if lsOpts.StartupTimeout != 7*time.Second || lsOpts.WSLMountFSType != "btrfs" {
		t.Fatalf("unexpected ls options: %+v", lsOpts)
	}

	rmOpts := ctx.rmOptions()
	if rmOpts.IdleTimeout != 90*time.Second || rmOpts.WSLVHDXPath != "/host/store" {
		t.Fatalf("unexpected rm options: %+v", rmOpts)
	}
}
