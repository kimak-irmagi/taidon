package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sqlrs/cli/internal/wsl"

	"golang.org/x/term"
)

type wslInitOptions struct {
	Enable      bool
	Distro      string
	Require     bool
	NoStart     bool
	Workspace   string
	Verbose     bool
	StoreSizeGB int
	Reinit      bool
	StorePath   string
}

type wslInitResult struct {
	UseWSL          bool
	Distro          string
	StateDir        string
	EnginePath      string
	StorePath       string
	MountDevice     string
	MountFSType     string
	MountUnit       string
	MountDeviceUUID string
	Warning         string
}

var initWSLFn = initWSL
var listWSLDistrosFn = listWSLDistros
var runWSLCommandFn = runWSLCommand
var runWSLCommandAllowFailureFn = runWSLCommandAllowFailure
var runWSLCommandWithInputFn = runWSLCommandWithInput
var runHostCommandFn = runHostCommand
var isElevatedFn = isElevated
var isWindows = runtime.GOOS == "windows"

const defaultVHDXName = "btrfs.vhdx"

func initWSL(opts wslInitOptions) (wslInitResult, error) {
	if !opts.Enable {
		return wslInitResult{}, nil
	}
	if !isWindows {
		return wslInitResult{}, fmt.Errorf("WSL init is only supported on Windows")
	}

	logWSLInit(opts.Verbose, "checking WSL availability")
	if _, err := exec.LookPath("wsl.exe"); err != nil {
		return wslUnavailable(opts, "WSL is not available")
	}

	logWSLInit(opts.Verbose, "listing WSL distros")
	distros, err := listWSLDistrosFn()
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL unavailable: %v", err))
	}
	distro, err := wsl.SelectDistro(distros, opts.Distro)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL distro resolution failed: %v", err))
	}
	logWSLInit(opts.Verbose, "selected WSL distro: %s", distro)

	if !opts.NoStart {
		if _, err := runWSLCommandFn(context.Background(), distro, opts.Verbose, "starting WSL distro", "true"); err != nil {
			return wslUnavailable(opts, fmt.Sprintf("WSL distro start failed: %v", err))
		}
	}

	if err := ensureBtrfsKernel(distro, opts.Verbose); err != nil {
		return wslUnavailable(opts, err.Error())
	}
	if err := ensureBtrfsProgs(distro, opts.Verbose); err != nil {
		return wslUnavailable(opts, err.Error())
	}
	if err := ensureNsenter(distro, opts.Verbose); err != nil {
		return wslUnavailable(opts, err.Error())
	}
	if err := ensureSystemdAvailable(distro, opts.Verbose); err != nil {
		return wslUnavailable(opts, err.Error())
	}

	warnings := []string{}
	dockerRunning, dockerWarning, err := checkDockerDesktopRunning(opts.Verbose)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Docker Desktop check failed: %v", err))
	} else if !dockerRunning {
		if dockerWarning != "" {
			warnings = append(warnings, dockerWarning)
		}
	} else {
		ok, warn := checkDockerInWSL(distro, opts.Verbose)
		if !ok && warn != "" {
			warnings = append(warnings, warn)
		}
	}

	storeDir, storePath, err := resolveHostStorePath()
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL store path resolution failed: %v", err))
	}
	if strings.TrimSpace(opts.StorePath) != "" {
		storePath = strings.TrimSpace(opts.StorePath)
		storeDir = filepath.Dir(storePath)
	}
	logWSLInit(opts.Verbose, "host store dir: %s", storeDir)
	logWSLInit(opts.Verbose, "host vhdx path: %s", storePath)

	sizeGB := opts.StoreSizeGB
	if sizeGB <= 0 {
		sizeGB = defaultBtrfsStoreSizeGB
	}

	ctx := context.Background()
	elevated, err := isElevatedFn(opts.Verbose)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("Administrator check failed: %v", err))
	}
	if !elevated {
		return wslUnavailable(opts, "WSL init requires Administrator privileges to create and mount VHDX. Please rerun this command in an elevated terminal (Run as Administrator).")
	}
	stateDir, err := resolveWSLStateStore(distro, opts.Verbose)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL state dir resolution failed: %v", err))
	}
	logWSLInit(opts.Verbose, "btrfs state dir: %s", stateDir)
	mountUnit, err := resolveSystemdMountUnit(ctx, distro, stateDir, opts.Verbose)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL mount unit resolution failed: %v", err))
	}
	logWSLInit(opts.Verbose, "systemd mount unit: %s", mountUnit)

	if opts.Reinit {
		if err := reinitWSLStore(ctx, distro, stateDir, storePath, mountUnit, opts.Verbose); err != nil {
			return wslUnavailable(opts, fmt.Sprintf("WSL reinit failed: %v", err))
		}
	}

	created, err := ensureHostVHDX(ctx, storePath, sizeGB, opts.Verbose)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("VHDX init failed: %v", err))
	}

	disk, part, err := findWSLDisk(ctx, distro, int64(sizeGB)*1024*1024*1024, opts.Verbose)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("WSL disk detection failed: %v", err))
	}
	attached := disk != ""

	if !attached {
		if err := ensureHostGPTPartition(ctx, storePath, opts.Verbose); err != nil {
			return wslUnavailable(opts, fmt.Sprintf("VHDX partitioning failed: %v", err))
		}
		if err := attachVHDXToWSL(ctx, storePath, distro, opts.Verbose); err != nil {
			return wslUnavailable(opts, fmt.Sprintf("WSL mount failed: %v", err))
		}
		disk, part, err = findWSLDisk(ctx, distro, int64(sizeGB)*1024*1024*1024, opts.Verbose)
		if err != nil {
			return wslUnavailable(opts, fmt.Sprintf("WSL disk detection failed: %v", err))
		}
	}

	logWSLInit(opts.Verbose, "WSL disk: %s", disk)
	if part == "" {
		if created || opts.Reinit {
			return wslUnavailable(opts, "WSL disk has no partition after initialization")
		}
		return wslUnavailable(opts, "WSL disk is missing required partition. Please rerun with --reinit.")
	}
	logWSLInit(opts.Verbose, "WSL partition: %s", part)

	allowFormat := created || opts.Reinit
	if err := ensureBtrfsOnPartition(ctx, distro, part, allowFormat, opts.Verbose); err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs format failed: %v", err))
	}
	deviceUUID, err := resolveWSLPartitionUUID(ctx, distro, part, opts.Verbose)
	if err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs device UUID failed: %v", err))
	}
	mountSource := part
	if deviceUUID != "" {
		mountSource = "/dev/disk/by-uuid/" + deviceUUID
		if exists, err := wslPathExists(ctx, distro, mountSource, opts.Verbose); err != nil {
			return wslUnavailable(opts, fmt.Sprintf("btrfs device path check failed: %v", err))
		} else if !exists {
			warnings = append(warnings, fmt.Sprintf("WSL mount: %s not found, using %s", mountSource, part))
			mountSource = part
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("WSL mount: partition UUID unavailable, using %s", part))
	}
	if err := installSystemdMountUnit(ctx, distro, mountUnit, stateDir, mountSource, "btrfs", opts.Verbose); err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs mount unit failed: %v", err))
	}
	if err := ensureSystemdMountUnitActive(ctx, distro, mountUnit, stateDir, "btrfs", opts.Verbose); err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs mount failed: %v", err))
	}
	if err := ensureBtrfsSubvolumes(ctx, distro, stateDir, opts.Verbose); err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs subvolumes failed: %v", err))
	}
	if err := ensureBtrfsOwnership(ctx, distro, stateDir, opts.Verbose); err != nil {
		return wslUnavailable(opts, fmt.Sprintf("btrfs ownership failed: %v", err))
	}
	warnings = append(warnings, "WSL restart required: wsl.exe --shutdown")

	return wslInitResult{
		UseWSL:          true,
		Distro:          distro,
		StateDir:        stateDir,
		StorePath:       storePath,
		MountDevice:     part,
		MountFSType:     "btrfs",
		MountUnit:       mountUnit,
		MountDeviceUUID: deviceUUID,
		Warning:         strings.Join(warnings, "\n"),
	}, nil
}

func listWSLDistros() ([]wsl.Distro, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "wsl.exe", "--list", "--verbose")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return wsl.ParseDistroList(string(out))
}

func ensureBtrfsKernel(distro string, verbose bool) error {
	out, err := runWSLCommandFn(context.Background(), distro, verbose, "check btrfs kernel", "cat", "/proc/filesystems")
	if err != nil {
		return fmt.Errorf("btrfs kernel check failed: %v", err)
	}
	if !strings.Contains(out, "btrfs") {
		_, _ = runWSLCommandFn(context.Background(), distro, verbose, "load btrfs module (root)", "modprobe", "btrfs")
		out, err = runWSLCommandFn(context.Background(), distro, verbose, "check btrfs kernel", "cat", "/proc/filesystems")
		if err != nil {
			return fmt.Errorf("btrfs kernel check failed: %v", err)
		}
		if !strings.Contains(out, "btrfs") {
			return fmt.Errorf("btrfs kernel support missing")
		}
	}
	return nil
}

func ensureBtrfsProgs(distro string, verbose bool) error {
	_, err := runWSLCommandFn(context.Background(), distro, verbose, "check btrfs-progs", "which", "mkfs.btrfs")
	if err == nil {
		return nil
	}
	logWSLInit(verbose, "installing btrfs-progs")
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(updateCtx, distro, verbose, "apt-get update (root)", "apt-get", "update"); err != nil {
		return fmt.Errorf("btrfs-progs install failed: %v", err)
	}
	installCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(installCtx, distro, verbose, "apt-get install (root)", "apt-get", "install", "-y", "btrfs-progs"); err != nil {
		return fmt.Errorf("btrfs-progs install failed: %v", err)
	}
	return nil
}

func ensureNsenter(distro string, verbose bool) error {
	_, err := runWSLCommandFn(context.Background(), distro, verbose, "check nsenter", "which", "nsenter")
	if err == nil {
		return nil
	}
	logWSLInit(verbose, "installing nsenter (util-linux)")
	updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(updateCtx, distro, verbose, "apt-get update (root)", "apt-get", "update"); err != nil {
		return fmt.Errorf("nsenter install failed: %v", err)
	}
	installCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := runWSLCommandFn(installCtx, distro, verbose, "apt-get install (root)", "apt-get", "install", "-y", "util-linux"); err != nil {
		return fmt.Errorf("nsenter install failed: %v", err)
	}
	return nil
}

func ensureSystemdAvailable(distro string, verbose bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runWSLCommandAllowFailureFn(ctx, distro, verbose, "check systemd (root)", "systemctl", "is-system-running")
	state := strings.TrimSpace(out)
	switch state {
	case "running", "degraded":
		return nil
	default:
		if err != nil && state == "degraded" {
			return nil
		}
	}
	if err != nil {
		return fmt.Errorf("systemd is not running in WSL distro %s (enable systemd and restart WSL)", distro)
	}
	if state == "" {
		state = "unknown"
	}
	return fmt.Errorf("systemd is not running in WSL distro %s (state=%s). Enable systemd and restart WSL", distro, state)
}

func checkDockerDesktopRunning(verbose bool) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runHostCommandFn(ctx, verbose, "check docker desktop", "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-Service -Name com.docker.service -ErrorAction SilentlyContinue).Status",
	)
	if err != nil {
		return false, "", err
	}
	status := strings.TrimSpace(out)
	if status == "" {
		if ok := checkDockerPipe(ctx, verbose); ok {
			return true, "", nil
		}
		if ok := checkDockerCLI(ctx, verbose); ok {
			return true, "", nil
		}
		return false, "Docker Desktop is not running (service not found)", nil
	}
	if strings.EqualFold(status, "Running") {
		return true, "", nil
	}
	if ok := checkDockerPipe(ctx, verbose); ok {
		return true, "", nil
	}
	if ok := checkDockerCLI(ctx, verbose); ok {
		return true, "", nil
	}
	return false, "Docker Desktop is not running", nil
}

func checkDockerPipe(ctx context.Context, verbose bool) bool {
	out, err := runHostCommandFn(ctx, verbose, "check docker pipe", "powershell", "-NoProfile", "-NonInteractive", "-Command",
		"[System.IO.File]::Exists('\\\\.\\pipe\\docker_engine')",
	)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(out), "True")
}

func checkDockerCLI(ctx context.Context, verbose bool) bool {
	_, err := runHostCommandFn(ctx, verbose, "check docker cli", "docker", "info")
	return err == nil
}

func checkDockerInWSL(distro string, verbose bool) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := runWSLCommandFn(ctx, distro, verbose, "check docker in WSL", "docker", "info")
	if err == nil {
		return true, ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "command not found"):
		return false, fmt.Sprintf("docker is not installed in WSL distro %s", distro)
	case strings.Contains(msg, "cannot connect to the docker daemon"),
		strings.Contains(msg, "is the docker daemon running"):
		return false, fmt.Sprintf("docker is not available in WSL distro %s. Enable Docker Desktop WSL integration and ensure Docker Desktop is running.", distro)
	default:
		return false, fmt.Sprintf("docker is not available in WSL distro %s: %v", distro, err)
	}
}

func resolveHostStorePath() (string, string, error) {
	storeDir := strings.TrimSpace(os.Getenv("SQLRS_STATE_STORE"))
	if storeDir == "" {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", "", err
			}
			local = filepath.Join(home, "AppData", "Local")
		}
		storeDir = filepath.Join(local, "sqlrs", "store")
	}
	storePath := filepath.Join(storeDir, defaultVHDXName)
	return storeDir, storePath, nil
}

func ensureHostVHDX(ctx context.Context, vhdxPath string, sizeGB int, verbose bool) (bool, error) {
	if vhdxPath == "" {
		return false, fmt.Errorf("vhdx path is empty")
	}
	if _, err := os.Stat(vhdxPath); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(vhdxPath), 0o700); err != nil {
		return false, err
	}
	sizeBytes := int64(sizeGB) * 1024 * 1024 * 1024
	cmd := fmt.Sprintf("New-VHD -Path '%s' -Dynamic -SizeBytes %d | Out-Null", escapePowerShellString(vhdxPath), sizeBytes)
	_, err := runHostCommandFn(ctx, verbose, "create VHDX", "powershell", "-NoProfile", "-NonInteractive", "-Command", cmd)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ensureHostGPTPartition(ctx context.Context, vhdxPath string, verbose bool) error {
	escapedPath := escapePowerShellString(vhdxPath)
	script := strings.Join([]string{
		"$path = '" + escapedPath + "';",
		"$attached = $false;",
		"$diskImage = Get-DiskImage -ImagePath $path -ErrorAction SilentlyContinue;",
		"if ($diskImage -and $diskImage.Attached) { $attached = $true; $disk = $diskImage | Get-Disk };",
		"if (-not $disk) { $vhd = Mount-VHD -Path $path -PassThru; $disk = $vhd | Get-Disk };",
		"if ($disk.PartitionStyle -eq 'RAW') { Initialize-Disk -Number $disk.Number -PartitionStyle GPT | Out-Null };",
		"$part = Get-Partition -DiskNumber $disk.Number | Where-Object { $_.Type -ne 'Reserved' } | Select-Object -First 1;",
		"if (-not $part) { New-Partition -DiskNumber $disk.Number -UseMaximumSize | Out-Null };",
		"if (-not $attached) { Dismount-VHD -Path $path -ErrorAction SilentlyContinue | Out-Null };",
	}, " ")
	_, err := runHostCommandFn(ctx, verbose, "partition VHDX", "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil && isVHDXInUseError(err) {
		return fmt.Errorf("VHDX is in use. Please rerun with --reinit or detach it from WSL")
	}
	return err
}

func attachVHDXToWSL(ctx context.Context, vhdxPath, distro string, verbose bool) error {
	_, err := runHostCommandFn(ctx, verbose, "attach VHDX to WSL", "wsl.exe", "--mount", vhdxPath, "--vhd", "--bare")
	return err
}

func resolveWSLStateStore(distro string, verbose bool) (string, error) {
	stateHome, err := runWSLCommandFn(context.Background(), distro, verbose, "resolve XDG_STATE_HOME", "printenv", "XDG_STATE_HOME")
	if err != nil {
		stateHome = ""
	}
	stateHome = sanitizeWSLOutput([]byte(stateHome))
	if stateHome == "" {
		home, err := runWSLCommandFn(context.Background(), distro, verbose, "resolve HOME", "printenv", "HOME")
		if err != nil {
			return "", fmt.Errorf("HOME is empty")
		}
		home = sanitizeWSLOutput([]byte(home))
		if home == "" {
			return "", fmt.Errorf("HOME is empty")
		}
		stateHome = path.Join(home, ".local", "state")
	}
	return path.Join(stateHome, "sqlrs", "store"), nil
}

func resolveSystemdMountUnit(ctx context.Context, distro, stateDir string, verbose bool) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	out, err := runWSLCommandFn(ctx, distro, verbose, "resolve mount unit (root)", "systemd-escape", "--path", "--suffix=mount", stateDir)
	if err != nil {
		return "", err
	}
	unit := sanitizeWSLOutput([]byte(out))
	if unit == "" {
		return "", fmt.Errorf("systemd mount unit is empty")
	}
	return unit, nil
}

func resolveWSLPartitionUUID(ctx context.Context, distro, part string, verbose bool) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(5 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	var lastErr error
	for {
		out, err := runWSLCommandFn(ctx, distro, verbose, "resolve partition UUID (root)", "blkid", "-o", "value", "-s", "UUID", part)
		if err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "command not found") {
				return "", err
			}
		} else {
			uuid := strings.TrimSpace(out)
			if uuid != "" {
				return uuid, nil
			}
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return "", fmt.Errorf("partition UUID unavailable: %w", lastErr)
			}
			return "", nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

type lsblkEntry struct {
	Name   string
	Size   int64
	Type   string
	PKName string
}

func findWSLDisk(ctx context.Context, distro string, sizeBytes int64, verbose bool) (string, string, error) {
	out, err := runWSLCommandFn(ctx, distro, verbose, "lsblk", "lsblk", "-b", "-n", "-r", "-o", "NAME,SIZE,TYPE,PKNAME")
	if err != nil {
		return "", "", err
	}
	entries, err := parseLsblk(out)
	if err != nil {
		return "", "", err
	}
	disk, err := selectDiskBySize(entries, sizeBytes)
	if err != nil {
		return "", "", err
	}
	if disk == "" {
		return "", "", nil
	}
	part, err := selectPartition(entries, disk)
	if err != nil {
		return "/dev/" + disk, "", nil
	}
	return "/dev/" + disk, "/dev/" + part, nil
}

func parseLsblk(output string) ([]lsblkEntry, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var entries []lsblkEntry
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] == "NAME" {
			continue
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid size: %s", fields[1])
		}
		entry := lsblkEntry{
			Name: cleanLsblkName(fields[0]),
			Size: size,
			Type: fields[2],
		}
		if len(fields) >= 4 {
			entry.PKName = cleanLsblkName(fields[3])
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no lsblk entries")
	}
	return entries, nil
}

func cleanLsblkName(value string) string {
	if value == "" {
		return value
	}
	trimmed := strings.TrimLeft(value, "├─└─│ ")
	for trimmed != value {
		value = trimmed
		trimmed = strings.TrimLeft(value, "├─└─│ ")
	}
	return value
}

func selectDiskBySize(entries []lsblkEntry, sizeBytes int64) (string, error) {
	var candidates []lsblkEntry
	for _, entry := range entries {
		if entry.Type != "disk" {
			continue
		}
		delta := entry.Size - sizeBytes
		if delta < 0 {
			delta = -delta
		}
		tolerance := int64(100 * 1024 * 1024)
		if sizeBytes > 0 {
			pct := sizeBytes / 100
			if pct > tolerance {
				tolerance = pct
			}
		}
		if delta <= tolerance {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		return "", nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("multiple disks match size")
	}
	return candidates[0].Name, nil
}

func selectPartition(entries []lsblkEntry, disk string) (string, error) {
	var selected *lsblkEntry
	for i := range entries {
		entry := &entries[i]
		if entry.Type != "part" || entry.PKName != disk {
			continue
		}
		if selected == nil || entry.Size > selected.Size {
			selected = entry
		}
	}
	if selected == nil {
		return "", fmt.Errorf("partition for disk %s not found", disk)
	}
	return selected.Name, nil
}

func ensureBtrfsOnPartition(ctx context.Context, distro, part string, allowFormat bool, verbose bool) error {
	mountedFs, mounted, err := wslFindmntFSType(ctx, distro, part, verbose)
	if err != nil {
		return err
	}
	if mounted {
		mountedFS := normalizeFSType(mountedFs)
		if mountedFS == "btrfs" {
			return nil
		}
		if mountedFS != "" {
			return fmt.Errorf("filesystem is %s, expected btrfs (rerun with --reinit)", mountedFS)
		}
		return fmt.Errorf("filesystem is not btrfs (rerun with --reinit)")
	}

	out, err := runWSLCommandFn(ctx, distro, verbose, "detect filesystem", "blkid", "-o", "value", "-s", "TYPE", part)
	if err == nil {
		fsType := normalizeFSType(out)
		if fsType == "btrfs" {
			return nil
		}
		if fsType != "" && !allowFormat {
			return fmt.Errorf("filesystem is %s, expected btrfs (rerun with --reinit)", fsType)
		}
		if fsType == "" {
			allowFormat = true
		}
	}
	if !allowFormat {
		return fmt.Errorf("filesystem is not btrfs (rerun with --reinit)")
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "wipefs (root)", "wipefs", "-a", part); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "command not found") {
			return err
		}
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "format btrfs (root)", "mkfs.btrfs", "-f", part); err != nil {
		return err
	}
	if err := waitForPartitionFSType(ctx, distro, part, "btrfs", verbose); err != nil {
		return err
	}
	return nil
}

func normalizeFSType(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func installSystemdMountUnit(ctx context.Context, distro, unitName, stateDir, mountSource, fstype string, verbose bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if unitName == "" {
		return fmt.Errorf("mount unit name is empty")
	}
	if stateDir == "" {
		return fmt.Errorf("mount state dir is empty")
	}
	if mountSource == "" {
		return fmt.Errorf("mount source is empty")
	}
	if fstype == "" {
		fstype = "btrfs"
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "create state dir", "mkdir", "-p", stateDir); err != nil {
		return err
	}
	unitPath := path.Join("/etc/systemd/system", unitName)
	content := strings.Join([]string{
		"[Unit]",
		"Description=SQLRS state store",
		"After=local-fs.target",
		"",
		"[Mount]",
		"What=" + mountSource,
		"Where=" + stateDir,
		"Type=" + fstype,
		"Options=defaults",
		"",
		"[Install]",
		"WantedBy=multi-user.target",
		"",
	}, "\n")
	if _, err := runWSLCommandWithInputFn(ctx, distro, verbose, "write mount unit (root)", content, "tee", unitPath); err != nil {
		return err
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "reload systemd (root)", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "enable mount unit (root)", "systemctl", "enable", unitName); err != nil {
		return err
	}
	return nil
}

func ensureSystemdMountUnitActive(ctx context.Context, distro, unitName, stateDir, fstype string, verbose bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if unitName == "" {
		return fmt.Errorf("mount unit name is empty")
	}
	if stateDir == "" {
		return fmt.Errorf("mount state dir is empty")
	}
	if fstype == "" {
		fstype = "btrfs"
	}
	active := false
	out, err := runWSLCommandFn(ctx, distro, verbose, "check mount unit (root)", "systemctl", "is-active", unitName)
	if err == nil && strings.TrimSpace(out) == "active" {
		active = true
	}
	if !active {
		if _, err := runWSLCommandFn(ctx, distro, verbose, "start mount unit (root)", "systemctl", "start", unitName); err != nil {
			if verbose {
				if tail, tailErr := runWSLCommandFn(ctx, distro, verbose, "mount unit logs (root)", "journalctl", "-u", unitName, "-n", "20", "--no-pager"); tailErr == nil {
					return fmt.Errorf("%v\n%s", err, strings.TrimSpace(tail))
				}
			}
			return err
		}
		out, err = runWSLCommandFn(ctx, distro, verbose, "check mount unit (root)", "systemctl", "is-active", unitName)
		if err != nil || strings.TrimSpace(out) != "active" {
			return fmt.Errorf("mount unit is not active")
		}
	}
	if err := waitForMountFSType(ctx, distro, stateDir, fstype, verbose); err != nil {
		return err
	}
	return nil
}

func waitForPartitionFSType(ctx context.Context, distro, part, fstype string, verbose bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	attempts := 5
	var lastFS string
	var lastErr error
	for i := 0; i < attempts; i++ {
		out, err := runWSLCommandFn(ctx, distro, verbose, "verify filesystem (root)", "blkid", "-c", "/dev/null", "-p", "-o", "value", "-s", "TYPE", part)
		if err != nil {
			lastErr = err
		} else {
			lastFS = strings.TrimSpace(out)
			if lastFS == fstype {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastFS == "" {
		if err := probeBtrfsMount(ctx, distro, part, verbose); err == nil {
			return nil
		} else if lastErr == nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		return fmt.Errorf("filesystem verification failed: %w", lastErr)
	}
	if lastFS == "" {
		return fmt.Errorf("filesystem verification failed: empty type")
	}
	return fmt.Errorf("filesystem verification failed: %s", lastFS)
}

func probeBtrfsMount(ctx context.Context, distro, part string, verbose bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	mountDir, err := runWSLCommandFn(ctx, distro, verbose, "probe mount dir (root)", "mktemp", "-d", "/tmp/sqlrs-mount-XXXXXX")
	if err != nil {
		return err
	}
	mountDir = strings.TrimSpace(mountDir)
	if mountDir == "" {
		return fmt.Errorf("probe mount dir is empty")
	}
	_, mountErr := runWSLCommandInInitNamespace(ctx, distro, verbose, "probe mount (root)", "mount", "-t", "btrfs", part, mountDir)
	if mountErr != nil {
		_, _ = runWSLCommandFn(ctx, distro, verbose, "cleanup probe dir (root)", "rmdir", mountDir)
		return mountErr
	}
	_, umountErr := runWSLCommandInInitNamespace(ctx, distro, verbose, "probe umount (root)", "umount", mountDir)
	_, _ = runWSLCommandFn(ctx, distro, verbose, "cleanup probe dir (root)", "rmdir", mountDir)
	return umountErr
}

func waitForMountFSType(ctx context.Context, distro, stateDir, fstype string, verbose bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	attempts := 5
	var lastFS string
	var mounted bool
	var lastErr error
	for i := 0; i < attempts; i++ {
		fsType, isMounted, err := wslFindmntFSType(ctx, distro, stateDir, verbose)
		if err != nil {
			lastErr = err
		} else {
			mounted = isMounted
			lastFS = strings.TrimSpace(fsType)
			if mounted && lastFS == fstype {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	if !mounted {
		return fmt.Errorf("mount verification failed for %s", stateDir)
	}
	return fmt.Errorf("mounted filesystem is %s, expected %s", lastFS, fstype)
}

func removeSystemdMountUnit(ctx context.Context, distro, unitName string, verbose bool) {
	if unitName == "" {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "stop mount unit (root)", "systemctl", "stop", unitName); err != nil {
		logWSLInit(verbose, "systemd stop failed (ignored): %v", err)
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "disable mount unit (root)", "systemctl", "disable", unitName); err != nil {
		logWSLInit(verbose, "systemd disable failed (ignored): %v", err)
	}
	unitPath := path.Join("/etc/systemd/system", unitName)
	if _, err := runWSLCommandFn(ctx, distro, verbose, "remove mount unit (root)", "rm", "-f", unitPath); err != nil {
		logWSLInit(verbose, "systemd unit removal failed (ignored): %v", err)
	}
	if _, err := runWSLCommandFn(ctx, distro, verbose, "reload systemd (root)", "systemctl", "daemon-reload"); err != nil {
		logWSLInit(verbose, "systemd reload failed (ignored): %v", err)
	}
}

func ensureBtrfsSubvolumes(ctx context.Context, distro, stateDir string, verbose bool) error {
	subvols := []string{"@instances", "@states"}
	for _, sub := range subvols {
		target := path.Join(stateDir, sub)
		_, err := runWSLCommandInInitNamespace(ctx, distro, verbose, "check path", "stat", target)
		if err != nil {
			if strings.Contains(err.Error(), "No such file") || strings.Contains(err.Error(), "cannot stat") || strings.Contains(err.Error(), "exit status 1") {
				// continue to create
			} else {
				return err
			}
		} else {
			continue
		}
		if _, err := runWSLCommandInInitNamespace(ctx, distro, verbose, "create subvolume (root)", "btrfs", "subvolume", "create", target); err != nil {
			return err
		}
	}
	return nil
}

func ensureBtrfsOwnership(ctx context.Context, distro, stateDir string, verbose bool) error {
	user, group, err := resolveWSLUser(distro, verbose)
	if err != nil {
		return err
	}
	owner := user
	if group != "" {
		owner = user + ":" + group
	}
	if _, err := runWSLCommandInInitNamespace(ctx, distro, verbose, "chown btrfs (root)", "chown", "-R", owner, stateDir); err != nil {
		return err
	}
	return nil
}

func resolveWSLUser(distro string, verbose bool) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	userOut, err := runWSLCommandFn(ctx, distro, verbose, "resolve WSL user", "id", "-un")
	if err != nil {
		return "", "", err
	}
	user := strings.TrimSpace(userOut)
	if user == "" {
		return "", "", fmt.Errorf("WSL user is empty")
	}
	groupOut, err := runWSLCommandFn(ctx, distro, verbose, "resolve WSL group", "id", "-gn")
	if err != nil {
		return user, "", nil
	}
	group := strings.TrimSpace(groupOut)
	return user, group, nil
}

func reinitWSLStore(ctx context.Context, distro, stateDir, vhdxPath, mountUnit string, verbose bool) error {
	removeSystemdMountUnit(ctx, distro, mountUnit, verbose)

	fsType, mounted, err := wslFindmntFSType(ctx, distro, stateDir, verbose)
	if err != nil {
		return err
	}
	if mounted {
		if strings.TrimSpace(fsType) != "" {
			logWSLInit(verbose, "unmounting previous WSL store (%s)", strings.TrimSpace(fsType))
		}
		if _, err := runWSLCommandInInitNamespace(ctx, distro, verbose, "unmount btrfs (root)", "umount", stateDir); err != nil {
			if !isWSLNotMountedError(err) {
				return err
			}
		}
	}

	if _, err := runHostCommandFn(ctx, verbose, "unmount VHDX from WSL", "wsl.exe", "--unmount", vhdxPath); err != nil {
		logWSLInit(verbose, "wsl unmount failed (ignored): %v", err)
	}

	script := strings.Join([]string{
		"$path = '" + escapePowerShellString(vhdxPath) + "';",
		"$diskImage = Get-DiskImage -ImagePath $path -ErrorAction SilentlyContinue;",
		"if ($diskImage -and $diskImage.Attached) { Dismount-VHD -Path $path -ErrorAction SilentlyContinue | Out-Null };",
	}, " ")
	if _, err := runHostCommandFn(ctx, verbose, "unmount VHDX on host", "powershell", "-NoProfile", "-NonInteractive", "-Command", script); err != nil {
		logWSLInit(verbose, "host VHDX unmount failed (ignored): %v", err)
	}

	if err := os.Remove(vhdxPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func wslPathExists(ctx context.Context, distro, pathValue string, verbose bool) (bool, error) {
	_, err := runWSLCommandFn(ctx, distro, verbose, "check path", "stat", pathValue)
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "No such file") || strings.Contains(err.Error(), "cannot stat") || strings.Contains(err.Error(), "exit status 1") {
		return false, nil
	}
	return false, err
}

func wslMountpoint(ctx context.Context, distro, pathValue string, verbose bool) (bool, error) {
	_, err := runWSLCommandFn(ctx, distro, verbose, "check mountpoint", "mountpoint", "-q", pathValue)
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "not a mountpoint") ||
		strings.Contains(err.Error(), "No such file") ||
		strings.Contains(err.Error(), "exit status 1") ||
		strings.Contains(err.Error(), "exit status 32") {
		return false, nil
	}
	return false, err
}

func wslFindmntFSType(ctx context.Context, distro, target string, verbose bool) (string, bool, error) {
	args := []string{"-n", "-o", "FSTYPE"}
	if strings.HasPrefix(strings.TrimSpace(target), "/dev/") {
		args = append(args, "-S", target)
	} else {
		args = append(args, "-T", target)
	}
	out, err := wslFindmntRun(ctx, distro, verbose, args)
	if err == nil {
		fsType := strings.TrimSpace(out)
		if fsType == "" {
			return "", false, nil
		}
		return fsType, true, nil
	}
	if strings.Contains(err.Error(), "exit status 1") {
		return "", false, nil
	}
	return "", false, err
}

func wslFindmntRun(ctx context.Context, distro string, verbose bool, args []string) (string, error) {
	out, err := runWSLCommandInInitNamespace(ctx, distro, verbose, "findmnt (root)", "findmnt", args...)
	if err == nil {
		return out, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "command not found") {
		return runWSLCommandFn(ctx, distro, verbose, "findmnt (root)", "findmnt", args...)
	}
	return out, err
}

func runWSLCommandInInitNamespace(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
	cmdArgs := append([]string{"-t", "1", "-m", "--", command}, args...)
	out, err := runWSLCommandFn(ctx, distro, verbose, desc, "nsenter", cmdArgs...)
	if err == nil {
		return out, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "command not found") {
		return runWSLCommandFn(ctx, distro, verbose, desc, command, args...)
	}
	return out, err
}

func isWSLNotMountedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not mounted") || strings.Contains(msg, "exit status 32")
}

func wslUnavailable(opts wslInitOptions, warning string) (wslInitResult, error) {
	if opts.Require {
		return wslInitResult{}, errors.New(warning)
	}
	return wslInitResult{UseWSL: false, Warning: strings.TrimSpace(warning)}, nil
}

func sanitizeWSLOutput(data []byte) string {
	trimmed := strings.TrimSpace(string(data))
	if strings.Contains(trimmed, "\x00") {
		trimmed = strings.ReplaceAll(trimmed, "\x00", "")
		trimmed = strings.TrimSpace(trimmed)
	}
	return trimmed
}

func escapePowerShellString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func isVHDXInUseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "objectinuse") ||
		strings.Contains(msg, "in use by another process") ||
		strings.Contains(msg, "0x80070020")
}

func logWSLInit(verbose bool, format string, args ...any) {
	if !verbose {
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
	if !isTerminalWriter(os.Stderr) {
		return func() {}
	}

	done := make(chan struct{})
	shown := make(chan struct{})
	go func() {
		timer := time.NewTimer(500 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			close(shown)
		case <-done:
			return
		}
		spinner := []string{"-", "\\", "|", "/"}
		idx := 0
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				clearLine()
				return
			case <-ticker.C:
				clearLine()
				fmt.Fprintf(os.Stderr, "%s %s", label, spinner[idx])
				idx = (idx + 1) % len(spinner)
			}
		}
	}()
	return func() {
		close(done)
		select {
		case <-shown:
			clearLine()
		default:
		}
	}
}

func isTerminalWriter(w *os.File) bool {
	if w == nil {
		return false
	}
	return term.IsTerminal(int(w.Fd()))
}

func clearLine() {
	fmt.Fprint(os.Stderr, "\r\033[2K")
}
