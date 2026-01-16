package conntrack

import "context"

type Tracker interface {
	ActiveConnections(ctx context.Context, instanceID string) (int, error)
}
