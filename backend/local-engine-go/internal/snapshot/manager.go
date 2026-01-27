package snapshot

import "context"

type CloneResult struct {
	MountDir string
	Cleanup  func() error
}

type Manager interface {
	Kind() string
	Capabilities() Capabilities
	Clone(ctx context.Context, srcDir string, destDir string) (CloneResult, error)
	Snapshot(ctx context.Context, srcDir string, destDir string) error
	Destroy(ctx context.Context, dir string) error
}

type Capabilities struct {
	RequiresDBStop       bool
	SupportsWritableClone bool
	SupportsSendReceive   bool
}

type Options struct {
	PreferOverlay bool
}

func NewManager(opts Options) Manager {
	if opts.PreferOverlay && overlaySupportedFn() {
		return newOverlayManagerFn()
	}
	return CopyManager{}
}

var (
	overlaySupportedFn  = overlaySupported
	newOverlayManagerFn = newOverlayManager
)
