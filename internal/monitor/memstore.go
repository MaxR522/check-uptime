package monitor

import (
	"context"
	"slices"
	"sync"
)

// maxMemoryResults bounds the check results MemoryStore keeps.
const maxMemoryResults = 1000

// MemoryStore is an in-memory Store seeded with sample websites.
// It stands in for the SQLite store until that exists.
type MemoryStore struct {
	mu            sync.Mutex
	websites      []website
	openIncidents map[int][]incident // keyed by endpoint ID
	results       []checkResult
	nextIncident  int
	nextResult    int
}

func NewMemoryStore() *MemoryStore {
	health := endpoint{id: 1, path: "health", websiteID: 1, intervalCheckMin: 1, timeoutSec: 30, enabled: true}
	home := endpoint{id: 2, path: "home", websiteID: 1, intervalCheckMin: 2, timeoutSec: 30, enabled: true}
	root := endpoint{id: 3, path: "", websiteID: 1, intervalCheckMin: 2, timeoutSec: 30, enabled: true}
	localHealth := endpoint{id: 4, path: "health", websiteID: 2, intervalCheckMin: 1, timeoutSec: 30, enabled: true}
	localHome := endpoint{id: 5, path: "home", websiteID: 2, intervalCheckMin: 2, timeoutSec: 30, enabled: true}

	return &MemoryStore{
		websites: []website{
			{id: 1, name: "example", baseURL: "https://example.com", endpoints: []endpoint{health, home, root}},
			{id: 2, name: "local", baseURL: "http://127.0.0.1:1", endpoints: []endpoint{localHealth, localHome}},
		},
		openIncidents: make(map[int][]incident),
	}
}

func (s *MemoryStore) GetWebsites(ctx context.Context) ([]website, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.websites), nil
}

func (s *MemoryStore) OpenIncidents(ctx context.Context, endpointID int) ([]incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.openIncidents[endpointID]), nil
}

func (s *MemoryStore) StoreIncident(ctx context.Context, inc incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if inc.id == 0 {
		s.nextIncident++
		inc.id = s.nextIncident
	}

	open := slices.DeleteFunc(s.openIncidents[inc.endpointID], func(o incident) bool { return o.id == inc.id })
	if inc.resolvedAt.IsZero() {
		open = append(open, inc)
	}
	s.openIncidents[inc.endpointID] = open
	return nil
}

func (s *MemoryStore) StoreCheckResult(ctx context.Context, cr checkResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextResult++
	cr.id = s.nextResult
	s.results = append(s.results, cr)
	if len(s.results) > maxMemoryResults {
		s.results = slices.Delete(s.results, 0, len(s.results)-maxMemoryResults)
	}
	return nil
}
