package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWSLBtrfsProgsFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		switch desc {
		case "check btrfs-progs":
			return initWSLStubReply{Err: errInitTest("missing"), Handled: true}
		case "apt-get update (root)":
			return initWSLStubReply{Err: errInitTest("boom"), Handled: true}
		default:
			return initWSLStubReply{}
		}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "btrfs-progs") {
		t.Fatalf("expected btrfs-progs warning, got %+v", res)
	}
}

func TestInitWSLNsenterFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		switch desc {
		case "check nsenter":
			return initWSLStubReply{Err: errInitTest("missing"), Handled: true}
		case "apt-get update (root)":
			return initWSLStubReply{Err: errInitTest("boom"), Handled: true}
		default:
			return initWSLStubReply{}
		}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "nsenter") {
		t.Fatalf("expected nsenter warning, got %+v", res)
	}
}

func TestInitWSLSystemdUnavailableSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, nil, nil, true, nil)
	withRunWSLCommandAllowFailureStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errInitTest("boom")
	})

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "systemd") {
		t.Fatalf("expected systemd warning, got %+v", res)
	}
}

func TestInitWSLDockerDesktopCheckErrorSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, nil, func(desc, command string, args []string) initWSLStubReply {
		if desc == "check docker desktop" {
			return initWSLStubReply{Err: errInitTest("service query failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected success result, got %v", err)
	}
	if !res.UseWSL || !strings.Contains(res.Warning, "Docker Desktop check failed") {
		t.Fatalf("expected docker desktop warning, got %+v", res)
	}
}

func TestInitWSLDockerInWSLWarningSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "check docker in WSL" {
			return initWSLStubReply{Err: errInitTest("command not found"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected success result, got %v", err)
	}
	if !res.UseWSL || !strings.Contains(strings.ToLower(res.Warning), "docker is not installed") {
		t.Fatalf("expected docker-in-wsl warning, got %+v", res)
	}
}

func TestInitWSLSecondDiskDetectionFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	var lsblkCalls int
	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "lsblk" {
			lsblkCalls++
			if lsblkCalls == 1 {
				return initWSLStubReply{Out: "NAME SIZE TYPE PKNAME\nsda 1 disk\n", Handled: true}
			}
			return initWSLStubReply{Err: errInitTest("lsblk failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "disk detection failed") {
		t.Fatalf("expected disk detection warning, got %+v", res)
	}
}

func TestInitWSLMissingPartitionAfterCreateSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "lsblk" {
			return initWSLStubReply{Out: "NAME SIZE TYPE PKNAME\nsda 10737418240 disk\n", Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "no partition after initialization") {
		t.Fatalf("expected missing partition warning, got %+v", res)
	}
}

func TestInitWSLMountUnitInstallFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		if desc == "create state dir" {
			return initWSLStubReply{Err: errInitTest("mkdir failed"), Handled: true}
		}
		return initWSLStubReply{}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "mount unit failed") {
		t.Fatalf("expected mount unit warning, got %+v", res)
	}
}

func TestInitWSLMountUnitActiveFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		switch desc {
		case "check mount unit (root)":
			return initWSLStubReply{Out: "inactive\n", Handled: true}
		case "start mount unit (root)":
			return initWSLStubReply{Err: errInitTest("start failed"), Handled: true}
		default:
			return initWSLStubReply{}
		}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "btrfs mount failed") {
		t.Fatalf("expected mount active warning, got %+v", res)
	}
}

func TestInitWSLSubvolumesFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		switch desc {
		case "resolve partition UUID (root)":
			return initWSLStubReply{Out: "\n", Handled: true}
		case "check path":
			return initWSLStubReply{Err: errInitTest("permission denied"), Handled: true}
		default:
			return initWSLStubReply{}
		}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "subvolumes failed") {
		t.Fatalf("expected subvolumes warning, got %+v", res)
	}
}

func TestInitWSLOwnershipFailureSimulatedWindows(t *testing.T) {
	withWindowsMode(t)
	withFakeWslExe(t)

	withInitWSLBaseStubs(t, func(desc, command string, args []string) initWSLStubReply {
		switch desc {
		case "resolve partition UUID (root)":
			return initWSLStubReply{Out: "\n", Handled: true}
		case "chown btrfs (root)":
			return initWSLStubReply{Err: errInitTest("chown failed"), Handled: true}
		default:
			return initWSLStubReply{}
		}
	}, nil, true, nil)

	res, err := initWSL(defaultInitWSLOpts(t))
	if err != nil {
		t.Fatalf("expected warning result, got %v", err)
	}
	if res.UseWSL || !strings.Contains(res.Warning, "ownership failed") {
		t.Fatalf("expected ownership warning, got %+v", res)
	}
}

func TestResolveHostStorePathHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SQLRS_STATE_STORE", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	dir, path, err := resolveHostStorePath()
	if err != nil {
		t.Fatalf("resolveHostStorePath: %v", err)
	}
	expectedDir := filepath.Join(home, "AppData", "Local", "sqlrs", "store")
	if !strings.EqualFold(dir, expectedDir) {
		t.Fatalf("expected dir %q, got %q", expectedDir, dir)
	}
	if !strings.HasSuffix(path, defaultVHDXName) {
		t.Fatalf("expected path suffix %q, got %q", defaultVHDXName, path)
	}
}

func TestCheckDockerPipeAdditionalBranches(t *testing.T) {
	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", errInitTest("boom")
	})
	if checkDockerPipe(context.Background(), false) {
		t.Fatalf("expected false on host command error")
	}

	withRunHostCommandStub(t, func(ctx context.Context, verbose bool, desc string, command string, args ...string) (string, error) {
		return "False\n", nil
	})
	if checkDockerPipe(context.Background(), false) {
		t.Fatalf("expected false when pipe is absent")
	}
}

func TestWslPathExistsTrue(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", nil
	})

	ok, err := wslPathExists(context.Background(), "Ubuntu", "/mnt/store", false)
	if err != nil || !ok {
		t.Fatalf("expected existing path, ok=%v err=%v", ok, err)
	}
}

func TestWslMountpointTrue(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "", nil
	})

	ok, err := wslMountpoint(context.Background(), "Ubuntu", "/mnt/store", false)
	if err != nil || !ok {
		t.Fatalf("expected mountpoint, ok=%v err=%v", ok, err)
	}
}

func TestWslFindmntFSTypeEmptyOutput(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		return "\n", nil
	})

	fs, mounted, err := wslFindmntFSType(context.Background(), "Ubuntu", "/mnt/store", false)
	if err != nil || mounted || fs != "" {
		t.Fatalf("expected empty non-mounted result, fs=%q mounted=%v err=%v", fs, mounted, err)
	}
}

func TestWslFindmntRunSecondFallback(t *testing.T) {
	findmntCalls := 0
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if command == "nsenter" {
			return "", errInitTest("command not found")
		}
		if command == "findmnt" {
			findmntCalls++
			if findmntCalls == 1 {
				return "", errInitTest("command not found")
			}
			return "btrfs\n", nil
		}
		return "", nil
	})

	out, err := wslFindmntRun(context.Background(), "Ubuntu", false, []string{"-n", "-o", "FSTYPE", "-T", "/mnt/store"})
	if err != nil {
		t.Fatalf("wslFindmntRun: %v", err)
	}
	if strings.TrimSpace(out) != "btrfs" || findmntCalls != 2 {
		t.Fatalf("unexpected fallback result out=%q calls=%d", out, findmntCalls)
	}
}

func TestEnsureBtrfsSubvolumesUnexpectedError(t *testing.T) {
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		if desc == "check path" {
			return "", errInitTest("permission denied")
		}
		return "", nil
	})

	if err := ensureBtrfsSubvolumes(context.Background(), "Ubuntu", "/mnt/store", false); err == nil {
		t.Fatalf("expected unexpected stat error")
	}
}

func TestEnsureBtrfsOwnershipAdditionalErrors(t *testing.T) {
	t.Run("resolve-user-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			if desc == "resolve WSL user" {
				return "", errInitTest("id failed")
			}
			return "", nil
		})
		if err := ensureBtrfsOwnership(context.Background(), "Ubuntu", "/mnt/store", false); err == nil {
			t.Fatalf("expected resolve user error")
		}
	})

	t.Run("chown-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			switch desc {
			case "resolve WSL user":
				return "user\n", nil
			case "resolve WSL group":
				return "group\n", nil
			case "chown btrfs (root)":
				return "", errInitTest("chown failed")
			default:
				return "", nil
			}
		})
		if err := ensureBtrfsOwnership(context.Background(), "Ubuntu", "/mnt/store", false); err == nil {
			t.Fatalf("expected chown error")
		}
	})
}

func TestResolveWSLUserAdditionalErrors(t *testing.T) {
	t.Run("command-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			return "", errInitTest("id failed")
		})
		if _, _, err := resolveWSLUser("Ubuntu", false); err == nil {
			t.Fatalf("expected resolve user command error")
		}
	})

	t.Run("empty-user", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			switch desc {
			case "resolve WSL user":
				return "\n", nil
			case "resolve WSL group":
				return "group\n", nil
			default:
				return "", nil
			}
		})
		if _, _, err := resolveWSLUser("Ubuntu", false); err == nil {
			t.Fatalf("expected empty user error")
		}
	})
}

func TestRemoveSystemdMountUnitEmptyName(t *testing.T) {
	calls := 0
	withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
		calls++
		return "", nil
	})

	removeSystemdMountUnit(nil, "Ubuntu", "", true)
	if calls != 0 {
		t.Fatalf("expected no command calls for empty unit name")
	}
}

func TestEnsureBtrfsKernelAdditionalErrors(t *testing.T) {
	t.Run("initial-check-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			if desc == "check btrfs kernel" {
				return "", errInitTest("cat failed")
			}
			return "", nil
		})
		if err := ensureBtrfsKernel("Ubuntu", false); err == nil {
			t.Fatalf("expected initial check error")
		}
	})

	t.Run("second-check-error", func(t *testing.T) {
		checkCalls := 0
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			if desc == "check btrfs kernel" {
				checkCalls++
				if checkCalls == 1 {
					return "nodev ext4\n", nil
				}
				return "", errInitTest("cat failed")
			}
			return "", nil
		})
		if err := ensureBtrfsKernel("Ubuntu", false); err == nil {
			t.Fatalf("expected second check error")
		}
	})
}

func TestEnsureHostVHDXEmptyPath(t *testing.T) {
	created, err := ensureHostVHDX(context.Background(), "", 10, false)
	if err == nil || created {
		t.Fatalf("expected empty path error, created=%v err=%v", created, err)
	}
}

func TestInstallSystemdMountUnitAdditionalBranches(t *testing.T) {
	t.Run("default-fstype-and-nil-context", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			return "", nil
		})
		withRunWSLCommandWithInputStub(t, func(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
			if !strings.Contains(input, "Type=btrfs") {
				t.Fatalf("expected default btrfs fstype in unit file, got %q", input)
			}
			return "", nil
		})
		if err := installSystemdMountUnit(nil, "Ubuntu", "sqlrs.mount", "/mnt/store", "/dev/sda1", "", false); err != nil {
			t.Fatalf("installSystemdMountUnit: %v", err)
		}
	})

	t.Run("write-mount-unit-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			return "", nil
		})
		withRunWSLCommandWithInputStub(t, func(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
			return "", errInitTest("tee failed")
		})
		if err := installSystemdMountUnit(context.Background(), "Ubuntu", "sqlrs.mount", "/mnt/store", "/dev/sda1", "btrfs", false); err == nil {
			t.Fatalf("expected write mount unit error")
		}
	})

	t.Run("reload-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			if desc == "reload systemd (root)" {
				return "", errInitTest("reload failed")
			}
			return "", nil
		})
		withRunWSLCommandWithInputStub(t, func(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
			return "", nil
		})
		if err := installSystemdMountUnit(context.Background(), "Ubuntu", "sqlrs.mount", "/mnt/store", "/dev/sda1", "btrfs", false); err == nil {
			t.Fatalf("expected reload error")
		}
	})

	t.Run("enable-error", func(t *testing.T) {
		withRunWSLCommandStub(t, func(ctx context.Context, distro string, verbose bool, desc string, command string, args ...string) (string, error) {
			if desc == "enable mount unit (root)" {
				return "", errInitTest("enable failed")
			}
			return "", nil
		})
		withRunWSLCommandWithInputStub(t, func(ctx context.Context, distro string, verbose bool, desc string, input string, command string, args ...string) (string, error) {
			return "", nil
		})
		if err := installSystemdMountUnit(context.Background(), "Ubuntu", "sqlrs.mount", "/mnt/store", "/dev/sda1", "btrfs", false); err == nil {
			t.Fatalf("expected enable error")
		}
	})
}
