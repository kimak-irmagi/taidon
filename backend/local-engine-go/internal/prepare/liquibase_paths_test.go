package prepare

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

type fakeMapper struct {
	mapped map[string]string
	err    error
}

func (f fakeMapper) MapPath(path string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.mapped == nil {
		return path, nil
	}
	if value, ok := f.mapped[path]; ok {
		return value, nil
	}
	return path, nil
}

func TestMapLiquibaseArgsMapsPaths(t *testing.T) {
	mapper := fakeMapper{mapped: map[string]string{
		"/mnt/c/work/changelog.xml":    "C:\\work\\changelog.xml",
		"/mnt/c/work/props.properties": "C:\\work\\props.properties",
		"/mnt/c/work/db":               "C:\\work\\db",
	}}
	args := []string{
		"update",
		"--changelog-file", "/mnt/c/work/changelog.xml",
		"--defaults-file=/mnt/c/work/props.properties",
		"--searchPath", "/mnt/c/work/db,classpath:db",
	}
	out, err := mapLiquibaseArgs(args, mapper)
	if err != nil {
		t.Fatalf("mapLiquibaseArgs: %v", err)
	}
	if !containsArgPair(out, "--changelog-file", "C:\\work\\changelog.xml") {
		t.Fatalf("expected changelog mapped, got %+v", out)
	}
	if !containsArgValue(out, "--defaults-file=C:\\work\\props.properties") {
		t.Fatalf("expected defaults mapped, got %+v", out)
	}
	if !containsArgPair(out, "--searchPath", "C:\\work\\db,classpath:db") {
		t.Fatalf("expected searchPath mapped, got %+v", out)
	}
}

func TestMapLiquibaseArgsMissingValue(t *testing.T) {
	_, err := mapLiquibaseArgs([]string{"--changelog-file"}, fakeMapper{})
	if err == nil || !strings.Contains(err.Error(), "missing value for --changelog-file") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func TestMapLiquibaseArgsMapperError(t *testing.T) {
	_, err := mapLiquibaseArgs([]string{"--changelog-file", "/x"}, fakeMapper{err: errors.New("boom")})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected mapper error, got %v", err)
	}
}

func TestWslPathMapperKeepsRelativePath(t *testing.T) {
	setWSLForTest(t, true)
	mapper := wslPathMapper{}
	out, err := mapper.MapPath("liquibase\\changelog.xml")
	if err != nil {
		t.Fatalf("MapPath: %v", err)
	}
	if out != "liquibase\\changelog.xml" {
		t.Fatalf("expected relative path to stay, got %q", out)
	}
}

func TestWslPathMapperUsesWslpathM(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo C:/Users/Zlygo/work/file.xml")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo C:/Users/Zlygo/work/file.xml")
	}
	setWSLForTest(t, true)
	mapper := wslPathMapper{}
	out, err := mapper.MapPath("/mnt/c/Users/Zlygo/work/file.xml")
	if err != nil {
		t.Fatalf("MapPath: %v", err)
	}
	if out != "C:\\Users\\Zlygo\\work\\file.xml" {
		t.Fatalf("expected windows path, got %q", out)
	}
}

func TestNormalizeLiquibaseExecPath(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo /mnt/c/Tools/liquibase.exe")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo /mnt/c/Tools/liquibase.exe")
	}
	setWSLForTest(t, true)
	out, err := normalizeLiquibaseExecPath("C:\\Tools\\liquibase.exe", false)
	if err != nil {
		t.Fatalf("normalizeLiquibaseExecPath: %v", err)
	}
	if out != "/mnt/c/Tools/liquibase.exe" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestNormalizeLiquibaseExecPathNoWSL(t *testing.T) {
	setWSLForTest(t, false)
	out, err := normalizeLiquibaseExecPath("C:\\Tools\\liquibase.exe", false)
	if err != nil {
		t.Fatalf("normalizeLiquibaseExecPath: %v", err)
	}
	if out != "C:\\Tools\\liquibase.exe" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestLooksLikeWindowsPath(t *testing.T) {
	if !looksLikeWindowsPath("C:\\Tools") {
		t.Fatalf("expected windows path")
	}
	if looksLikeWindowsPath("/mnt/c/Tools") {
		t.Fatalf("expected non-windows path")
	}
}

func TestMapLiquibaseEnvWindowsExecKeepsPath(t *testing.T) {
	env := map[string]string{"JAVA_HOME": "C:\\Java"}
	out, err := mapLiquibaseEnv(env, true)
	if err != nil {
		t.Fatalf("mapLiquibaseEnv: %v", err)
	}
	if out["JAVA_HOME"] != "C:\\Java" {
		t.Fatalf("expected JAVA_HOME to stay windows, got %q", out["JAVA_HOME"])
	}
}

func TestMapLiquibaseEnvLinuxExecMapsPath(t *testing.T) {
	prev := execCommand
	t.Cleanup(func() {
		execCommand = prev
	})
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", "echo /mnt/c/Java")
		}
		return exec.CommandContext(ctx, "sh", "-c", "echo /mnt/c/Java")
	}
	setWSLForTest(t, true)
	env := map[string]string{"JAVA_HOME": "C:\\Java"}
	out, err := mapLiquibaseEnv(env, false)
	if err != nil {
		t.Fatalf("mapLiquibaseEnv: %v", err)
	}
	if out["JAVA_HOME"] != "/mnt/c/Java" {
		t.Fatalf("expected JAVA_HOME mapped, got %q", out["JAVA_HOME"])
	}
}

func TestDefaultLiquibasePathMapperWSL(t *testing.T) {
	setWSLForTest(t, true)
	if defaultLiquibasePathMapper() == nil {
		t.Fatalf("expected WSL path mapper")
	}
}

func TestDefaultLiquibasePathMapperNoWSL(t *testing.T) {
	setWSLForTest(t, false)
	if defaultLiquibasePathMapper() != nil {
		t.Fatalf("expected no mapper without WSL")
	}
}

func TestSanitizeLiquibaseExecPathQuotes(t *testing.T) {
	out := sanitizeLiquibaseExecPath(`\"C:\Tools\liquibase.exe\"`)
	if out != `C:\Tools\liquibase.exe` {
		t.Fatalf("expected sanitized exec path, got %q", out)
	}
	out = sanitizeLiquibaseExecPath(`'C:\Tools\liquibase.exe'`)
	if out != `C:\Tools\liquibase.exe` {
		t.Fatalf("expected sanitized single-quoted path, got %q", out)
	}
}

func TestMapLiquibasePathValueSearchPathEmptyEntry(t *testing.T) {
	_, err := mapLiquibasePathValue("--searchPath", " /tmp, ", fakeMapper{})
	if err == nil || !strings.Contains(err.Error(), "searchPath is empty") {
		t.Fatalf("expected searchPath empty error, got %v", err)
	}
}

func TestNormalizeWindowsPath(t *testing.T) {
	out := normalizeWindowsPath("C:/Tools/liquibase.exe")
	if out != `C:\Tools\liquibase.exe` {
		t.Fatalf("expected windows slashes normalized, got %q", out)
	}
	out = normalizeWindowsPath(`C:\Tools\liquibase.exe`)
	if out != `C:\Tools\liquibase.exe` {
		t.Fatalf("expected existing backslashes preserved, got %q", out)
	}
}

func containsArgPair(args []string, key string, value string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == key && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}

func containsArgValue(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func setWSLForTest(t *testing.T, enabled bool) {
	t.Helper()
	prevInterop := envGet("WSL_INTEROP")
	prevDistro := envGet("WSL_DISTRO_NAME")
	if enabled {
		_ = envSet("WSL_INTEROP", "1")
		_ = envSet("WSL_DISTRO_NAME", "Ubuntu")
	} else {
		_ = envSet("WSL_INTEROP", "")
		_ = envSet("WSL_DISTRO_NAME", "")
	}
	t.Cleanup(func() {
		_ = envSet("WSL_INTEROP", prevInterop)
		_ = envSet("WSL_DISTRO_NAME", prevDistro)
	})
}

func envGet(key string) string {
	return getEnv(key)
}

func envSet(key, value string) error {
	return setEnv(key, value)
}
