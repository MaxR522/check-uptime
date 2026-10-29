package monitor

import "context"

type Store interface {
	GetWebsites(ctx context.Context) ([]website, error)
	StoreIncident(ctx context.Context, incident incident) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{
		store,
	}
}

func (s *Service) GetWebsites(ctx context.Context) ([]website, error) {
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
		intervalCheckMin: 1,
		timeoutSec:       30,
		enabled:          true,
		expectations:     nil,
		incidents:        nil,
	}

	website1 := website{
		id:        1,
		name:      "example",
		baseURL:   "https://example.com",
		endpoints: []endpoint{endpoint1, endpoint2},
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

func (s *Service) SaveIncident(ctx context.Context, incident incident) error {
	return s.store.StoreIncident(ctx, incident)
}
