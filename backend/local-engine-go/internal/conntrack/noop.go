package conntrack

import "context"

type Noop struct{}

func (Noop) ActiveConnections(ctx context.Context, instanceID string) (int, error) {
	return 0, nil
}
