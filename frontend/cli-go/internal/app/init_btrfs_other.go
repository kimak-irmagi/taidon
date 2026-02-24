//go:build !linux

package app

func initLocalBtrfsStore(opts localBtrfsInitOptions) (localBtrfsInitResult, error) {
	return localBtrfsInitResult{StorePath: opts.StorePath}, nil
}
