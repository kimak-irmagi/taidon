package app

import (
	"runtime"
	"testing"
)

func withInitLocalBtrfsStub(t *testing.T, fn func(localBtrfsInitOptions) (localBtrfsInitResult, error)) {
	t.Helper()
	prev := initLocalBtrfsStoreFn
	initLocalBtrfsStoreFn = fn
	t.Cleanup(func() {
		initLocalBtrfsStoreFn = prev
	})
}

func stubBtrfsInitForTests(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		withInitWSLStub(t, func(opts wslInitOptions) (wslInitResult, error) {
			return wslInitResult{UseWSL: true}, nil
		})
		return
	}
	withInitLocalBtrfsStub(t, func(opts localBtrfsInitOptions) (localBtrfsInitResult, error) {
		return localBtrfsInitResult{StorePath: opts.StorePath}, nil
	})
}
