package monitor

import (
	"context"
	"time"
)

// Store persists websites, check results and incidents.
type Store interface {
	GetWebsites(ctx context.Context) ([]website, error)
	// OpenIncidents returns the unresolved incidents of an endpoint,
	// at most one per reason. Empty when the endpoint has none.
	OpenIncidents(ctx context.Context, endpointID int) ([]incident, error)
	StoreIncident(ctx context.Context, inc incident) error
	// ResolveIncidents resolves every open incident of an endpoint at the
	// given time and returns them with resolvedAt set. Empty when none were open.
	ResolveIncidents(ctx context.Context, endpointID int, at time.Time) ([]incident, error)
	StoreCheckResult(ctx context.Context, cr checkResult) error
}
