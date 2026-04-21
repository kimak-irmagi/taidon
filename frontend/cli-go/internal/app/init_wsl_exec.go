package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

func logWSLInit(verbose bool, format string, args ...any) {
	if !verbose {
		return
	}
	if runtime.GOOS != "windows" {
		return
	}
	fmt.Fprintf(os.Stderr, "wsl init: "+format+"\n", args...)
}

func runWSLCommand(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rootMode := strings.HasSuffix(desc, " (root)")
	if rootMode && len(desc) > len(" (root)") {
		desc = strings.TrimSuffix(desc, " (root)")
	}
	cmdArgs := []string{"-d", distro}
	if rootMode {
		cmdArgs = append(cmdArgs, "-u", "root")
	}
	cmdArgs = append(cmdArgs, "--", command)
	cmdArgs = append(cmdArgs, args...)
	if verbose {
		logWSLInit(true, "%s: wsl.exe %s", desc, strings.Join(cmdArgs, " "))
	}

	stop := startSpinner(desc, verbose)
	defer stop()

	cmd := exec.CommandContext(ctx, "wsl.exe", cmdArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return "", fmt.Errorf("%s: %v (%s)", desc, err, errText)
		}
		return "", fmt.Errorf("%s: %v", desc, err)
	}
	return string(out), nil
}

func runWSLCommandAllowFailure(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rootMode := strings.HasSuffix(desc, " (root)")
	if rootMode && len(desc) > len(" (root)") {
		desc = strings.TrimSuffix(desc, " (root)")
	}
	cmdArgs := []string{"-d", distro}
	if rootMode {
		cmdArgs = append(cmdArgs, "-u", "root")
	}
	cmdArgs = append(cmdArgs, "--", command)
	cmdArgs = append(cmdArgs, args...)
	if verbose {
		logWSLInit(true, "%s: wsl.exe %s", desc, strings.Join(cmdArgs, " "))
	}

	stop := startSpinner(desc, verbose)
	defer stop()

	cmd := exec.CommandContext(ctx, "wsl.exe", cmdArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return string(out), fmt.Errorf("%s: %v (%s)", desc, err, errText)
		}
		return string(out), fmt.Errorf("%s: %v", desc, err)
	}
	return string(out), nil
}

func runWSLCommandWithInput(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rootMode := strings.HasSuffix(desc, " (root)")
	if rootMode && len(desc) > len(" (root)") {
		desc = strings.TrimSuffix(desc, " (root)")
	}
	cmdArgs := []string{"-d", distro}
	if rootMode {
		cmdArgs = append(cmdArgs, "-u", "root")
	}
	cmdArgs = append(cmdArgs, "--", command)
	cmdArgs = append(cmdArgs, args...)
	if verbose {
		logWSLInit(true, "%s: wsl.exe %s", desc, strings.Join(cmdArgs, " "))
	}

	stop := startSpinner(desc, verbose)
	defer stop()

	cmd := exec.CommandContext(ctx, "wsl.exe", cmdArgs...)
	cmd.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return "", fmt.Errorf("%s: %v (%s)", desc, err, errText)
		}
		return "", fmt.Errorf("%s: %v", desc, err)
	}
	return string(out), nil
}

func runHostCommand(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if verbose {
		logWSLInit(true, "%s: %s %s", desc, command, strings.Join(args, " "))
	}

	stop := startSpinner(desc, verbose)
	defer stop()

	cmd := exec.CommandContext(ctx, command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return "", fmt.Errorf("%s: %v (%s)", desc, err, errText)
		}
		return "", fmt.Errorf("%s: %v", desc, err)
	}
	return string(out), nil
}

func isElevated(verbose bool) (bool, error) {
	out, err := runHostCommandFn(context.Background(), verbose, "check admin",
		"powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)",
	)
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(out)
	return strings.EqualFold(value, "True"), nil
}

func startSpinner(label string, verbose bool) func() {
	if verbose {
		return func() {}
	}
	if !isTerminalWriterFn(os.Stderr) {
		return func() {}
	}

	done := make(chan struct{})
	finished := make(chan struct{})
	clear := func() {
		clearLineOut(os.Stderr, len(label)+2)
	}
	var once sync.Once
	go func() {
		defer close(finished)
		timer := time.NewTimer(spinnerInitialDelay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-done:
			return
		}
		spinner := []string{"-", "\\", "|", "/"}
		idx := 0
		ticker := time.NewTicker(spinnerTickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				clear()
				return
			case <-ticker.C:
				clear()
				fmt.Fprintf(os.Stderr, "%s %s", label, spinner[idx])
				idx = (idx + 1) % len(spinner)
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
			<-finished
		})
	}
}

func isTerminalWriter(w *os.File) bool {
	if w == nil {
		return false
	}
	return term.IsTerminal(int(w.Fd()))
}

func clearLine() {
	clearLineOut(os.Stderr, 80)
}
