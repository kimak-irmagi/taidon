package app

import (
	"errors"
	"testing"
)

func TestResolveStoreTypeForPlatform(t *testing.T) {
	t.Run("explicit store type wins", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("auto", "DIR", "windows",
			func() (string, error) { return "", errors.New("unexpected") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "dir" {
			t.Fatalf("expected explicit dir, got %q err=%v", got, err)
		}
	})

	t.Run("copy and overlay use dir", func(t *testing.T) {
		for _, snapshot := range []string{"copy", "overlay"} {
			got, err := resolveStoreTypeForPlatform(snapshot, "", "windows",
				func() (string, error) { return "", errors.New("unexpected") },
				func(string) (bool, error) { return false, errors.New("unexpected") },
			)
			if err != nil || got != "dir" {
				t.Fatalf("snapshot=%s got=%q err=%v", snapshot, got, err)
			}
		}
	})

	t.Run("btrfs windows uses image", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("btrfs", "", "windows",
			func() (string, error) { return "", errors.New("unexpected") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "image" {
			t.Fatalf("expected image, got %q err=%v", got, err)
		}
	})

	t.Run("btrfs linux falls back to image when default root fails", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("btrfs", "", "linux",
			func() (string, error) { return "", errors.New("boom") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "image" {
			t.Fatalf("expected image fallback, got %q err=%v", got, err)
		}
	})

	t.Run("btrfs linux uses dir on btrfs root", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("btrfs", "", "linux",
			func() (string, error) { return "/state/store", nil },
			func(path string) (bool, error) {
				if path != "/state/store" {
					t.Fatalf("unexpected path %q", path)
				}
				return true, nil
			},
		)
		if err != nil || got != "dir" {
			t.Fatalf("expected dir, got %q err=%v", got, err)
		}
	})

	t.Run("btrfs linux falls back to image on non-btrfs root", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("btrfs", "", "linux",
			func() (string, error) { return "/state/store", nil },
			func(string) (bool, error) { return false, nil },
		)
		if err != nil || got != "image" {
			t.Fatalf("expected image fallback, got %q err=%v", got, err)
		}
	})

	t.Run("btrfs linux falls back to image on probe error", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("btrfs", "", "linux",
			func() (string, error) { return "/state/store", nil },
			func(string) (bool, error) { return false, errors.New("probe failed") },
		)
		if err != nil || got != "image" {
			t.Fatalf("expected image fallback, got %q err=%v", got, err)
		}
	})

	t.Run("btrfs other platforms use dir", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("btrfs", "", "darwin",
			func() (string, error) { return "", errors.New("unexpected") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "dir" {
			t.Fatalf("expected dir, got %q err=%v", got, err)
		}
	})

	t.Run("auto windows uses image", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("auto", "", "windows",
			func() (string, error) { return "", errors.New("unexpected") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "image" {
			t.Fatalf("expected image, got %q err=%v", got, err)
		}
	})

	t.Run("auto non windows uses dir", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("auto", "", "linux",
			func() (string, error) { return "", errors.New("unexpected") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "dir" {
			t.Fatalf("expected dir, got %q err=%v", got, err)
		}
	})

	t.Run("unknown snapshot defaults to dir", func(t *testing.T) {
		got, err := resolveStoreTypeForPlatform("mystery", "", "windows",
			func() (string, error) { return "", errors.New("unexpected") },
			func(string) (bool, error) { return false, errors.New("unexpected") },
		)
		if err != nil || got != "dir" {
			t.Fatalf("expected dir, got %q err=%v", got, err)
		}
	})
}
