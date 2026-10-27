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
	return s.store.GetWebsites(ctx)
}

func (s *Service) SaveIncident(ctx context.Context, incident incident) error {
	return s.store.StoreIncident(ctx, incident)
}
