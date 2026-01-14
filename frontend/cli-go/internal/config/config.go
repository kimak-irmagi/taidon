package config

import "sqlrs/cli/internal/paths"

type Config struct {
	DefaultProfile string                   `yaml:"defaultProfile"`
	Client         ClientConfig             `yaml:"client"`
	Orchestrator   OrchestratorConfig       `yaml:"orchestrator"`
	Profiles       map[string]ProfileConfig `yaml:"profiles"`
}

type ClientConfig struct {
	Timeout string `yaml:"timeout"`
	Retries int    `yaml:"retries"`
	Output  string `yaml:"output"`
}

type OrchestratorConfig struct {
	StartupTimeout string `yaml:"startupTimeout"`
	IdleTimeout    string `yaml:"idleTimeout"`
	RunDir         string `yaml:"runDir"`
	DaemonPath     string `yaml:"daemonPath"`
}

type ProfileConfig struct {
	Mode      string     `yaml:"mode"`
	Endpoint  string     `yaml:"endpoint"`
	Autostart bool       `yaml:"autostart"`
	Auth      AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
	Mode     string `yaml:"mode"`
	TokenEnv string `yaml:"tokenEnv"`
}

type LoadOptions struct {
	WorkingDir string
	Dirs       *paths.Dirs
}

type LoadedConfig struct {
	Config            Config
	Paths             paths.Dirs
	ProjectConfigPath string
}
