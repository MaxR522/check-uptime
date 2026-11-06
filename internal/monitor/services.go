package monitor

import (
	"context"
	"time"
)

type Store interface {
	GetWebsites(ctx context.Context) ([]website, error)
	StoreIncident(ctx context.Context, incident incident) error
	GetIncident(ctx context.Context, incidentID int) (incident, error)
	StoreCheckResult(ctx context.Context, cr checkResult) error
}

type service struct {
	store Store
}

func newService(store Store) *service {
	return &service{
		store,
	}
}

func (s *service) GetWebsites(ctx context.Context) ([]website, error) {
	// fake data to check if it is working
	endpoint1 := endpoint{
		id:               1,
		path:             "health",
		websiteID:        1,
		intervalCheckMin: 1,
		timeoutSec:       30,
		enabled:          true,
		expectations:     nil,
		incidents:        nil,
	}

	endpoint2 := endpoint{
		id:               2,
		path:             "home",
		websiteID:        1,
		intervalCheckMin: 2,
		timeoutSec:       30,
		enabled:          true,
		expectations:     nil,
		incidents:        nil,
	}

	endpoint3 := endpoint{
		id:               3,
		path:             "",
		websiteID:        1,
		intervalCheckMin: 2,
		timeoutSec:       30,
		enabled:          true,
		expectations:     nil,
		incidents:        nil,
	}

	website1 := website{
		id:        1,
		name:      "example",
		baseURL:   "https://example.com",
		endpoints: []endpoint{endpoint1, endpoint2, endpoint3},
	}

	website2 := website{
		id:        2,
		name:      "local",
		baseURL:   "http://127.0.0.1:1",
		endpoints: []endpoint{endpoint1, endpoint2},
	}

	websites := []website{website1, website2}

	return websites, nil

	// return s.store.GetWebsites(ctx)
}

func (s *service) SaveIncident(ctx context.Context, newIncident incident) error {
	return s.store.StoreIncident(ctx, newIncident)
}

func (s *service) GetIncident(ctx context.Context, endpointID int) (incident, error) {
	// return s.store.GetIncident(ctx, endpointID)

	incident := incident{
		id:         0,
		reason:     "404 Not Found",
		startedAt:  time.Now(),
		endpointID: 1,
	}

	return incident, nil
}

func (s *service) SaveResult(ctx context.Context, cr checkResult) error {
	return s.store.StoreCheckResult(ctx, cr)
}
