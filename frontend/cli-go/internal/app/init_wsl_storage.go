package app

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func prepareWSLStorage(ctx context.Context, deps wslInitDeps, opts wslInitOptions, distro string) (wslStoragePhase, error) {
	storeDir, storePath, err := resolveHostStorePath()
	if err != nil {
		return wslStoragePhase{}, fmt.Errorf("WSL store path resolution failed: %v", err)
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
	if ctx == nil {
		ctx = context.Background()
	}

	elevated, err := deps.isElevated(opts.Verbose)
	if err != nil {
		return wslStoragePhase{}, fmt.Errorf("Administrator check failed: %v", err)
	}
	if !elevated {
		return wslStoragePhase{}, fmt.Errorf("WSL init requires Administrator privileges to create and mount VHDX. Please rerun this command in an elevated terminal (Run as Administrator).")
	}

	stateDir, err := resolveWSLStateStore(distro, opts.Verbose)
	if err != nil {
		return wslStoragePhase{}, fmt.Errorf("WSL state dir resolution failed: %v", err)
	}
	logWSLInit(opts.Verbose, "btrfs state dir: %s", stateDir)
	mountUnit, err := resolveSystemdMountUnit(ctx, distro, stateDir, opts.Verbose)
	if err != nil {
		return wslStoragePhase{}, fmt.Errorf("WSL mount unit resolution failed: %v", err)
	}
	logWSLInit(opts.Verbose, "systemd mount unit: %s", mountUnit)

	if opts.Reinit {
		if err := reinitWSLStore(ctx, distro, stateDir, storePath, mountUnit, opts.Verbose); err != nil {
			return wslStoragePhase{}, fmt.Errorf("WSL reinit failed: %v", err)
		}
	}

	created, err := ensureHostVHDX(ctx, storePath, sizeGB, opts.Verbose)
	if err != nil {
		return wslStoragePhase{}, fmt.Errorf("VHDX init failed: %v", err)
	}

	disk, part, err := findWSLDisk(ctx, distro, int64(sizeGB)*1024*1024*1024, opts.Verbose)
	if err != nil {
		return wslStoragePhase{}, fmt.Errorf("WSL disk detection failed: %v", err)
	}
	if disk == "" {
		if err := ensureHostGPTPartition(ctx, storePath, opts.Verbose); err != nil {
			return wslStoragePhase{}, fmt.Errorf("VHDX partitioning failed: %v", err)
		}
		if err := attachVHDXToWSL(ctx, storePath, distro, opts.Verbose); err != nil {
			return wslStoragePhase{}, fmt.Errorf("WSL mount failed: %v", err)
		}
		disk, part, err = findWSLDisk(ctx, distro, int64(sizeGB)*1024*1024*1024, opts.Verbose)
		if err != nil {
			return wslStoragePhase{}, fmt.Errorf("WSL disk detection failed: %v", err)
		}
	}

	logWSLInit(opts.Verbose, "WSL disk: %s", disk)
	if part == "" {
		if created || opts.Reinit {
			return wslStoragePhase{}, fmt.Errorf("WSL disk has no partition after initialization")
		}
		return wslStoragePhase{}, fmt.Errorf("WSL disk is missing required partition. Please rerun with --reinit.")
	}
	logWSLInit(opts.Verbose, "WSL partition: %s", part)

	allowFormat := created || opts.Reinit
	if err := ensureBtrfsOnPartition(ctx, distro, part, allowFormat, opts.Verbose); err != nil {
		return wslStoragePhase{}, fmt.Errorf("btrfs format failed: %v", err)
	}

	return wslStoragePhase{
		Distro:    distro,
		StateDir:  stateDir,
		StorePath: storePath,
		MountUnit: mountUnit,
		Partition: part,
	}, nil
}

func finalizeWSLMount(ctx context.Context, _ wslInitDeps, opts wslInitOptions, storage wslStoragePhase) (wslMountPhase, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	deviceUUID, err := resolveWSLPartitionUUID(ctx, storage.Distro, storage.Partition, opts.Verbose)
	if err != nil {
		return wslMountPhase{}, fmt.Errorf("btrfs device UUID failed: %v", err)
	}

	warnings := []string{}
	mountSource := storage.Partition
	if deviceUUID != "" {
		mountSource = "/dev/disk/by-uuid/" + deviceUUID
		if exists, err := wslPathExists(ctx, storage.Distro, mountSource, opts.Verbose); err != nil {
			return wslMountPhase{}, fmt.Errorf("btrfs device path check failed: %v", err)
		} else if !exists {
			warnings = append(warnings, fmt.Sprintf("WSL mount: %s not found, using %s", mountSource, storage.Partition))
			mountSource = storage.Partition
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("WSL mount: partition UUID unavailable, using %s", storage.Partition))
	}

	if err := installSystemdMountUnit(ctx, storage.Distro, storage.MountUnit, storage.StateDir, mountSource, "btrfs", opts.Verbose); err != nil {
		return wslMountPhase{}, fmt.Errorf("btrfs mount unit failed: %v", err)
	}
	if err := ensureSystemdMountUnitActive(ctx, storage.Distro, storage.MountUnit, storage.StateDir, "btrfs", opts.Verbose); err != nil {
		return wslMountPhase{}, fmt.Errorf("btrfs mount failed: %v", err)
	}
	if err := ensureBtrfsSubvolumes(ctx, storage.Distro, storage.StateDir, opts.Verbose); err != nil {
		return wslMountPhase{}, fmt.Errorf("btrfs subvolumes failed: %v", err)
	}
	if err := ensureBtrfsOwnership(ctx, storage.Distro, storage.StateDir, opts.Verbose); err != nil {
		return wslMountPhase{}, fmt.Errorf("btrfs ownership failed: %v", err)
	}

	return wslMountPhase{DeviceUUID: deviceUUID, Warnings: warnings}, nil
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
		return fmt.Errorf("filesystem is %s, expected btrfs (rerun with --reinit)", mountedFS)
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
