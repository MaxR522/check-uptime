package monitor

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// fakeStore records incidents and results. openErr, storeErr and resolveErr
// make the matching method fail.
type fakeStore struct {
	open       map[int][]incident
	openErr    error
	storeErr   error
	resolveErr error
	incidents  []incident
	resolved   []incident
	results    []checkResult
}

func (f *fakeStore) GetWebsites(ctx context.Context) ([]website, error) { return nil, nil }

func (f *fakeStore) OpenIncidents(ctx context.Context, endpointID int) ([]incident, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return f.open[endpointID], nil
}

func (f *fakeStore) StoreIncident(ctx context.Context, inc incident) error {
	if f.storeErr != nil {
		return f.storeErr
	}
	f.incidents = append(f.incidents, inc)
	return nil
}

func (f *fakeStore) ResolveIncidents(ctx context.Context, endpointID int, at time.Time) ([]incident, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	resolved := slices.Clone(f.open[endpointID])
	for i := range resolved {
		resolved[i].resolvedAt = at
	}
	delete(f.open, endpointID)
	f.resolved = append(f.resolved, resolved...)
	return resolved, nil
}

func (f *fakeStore) StoreCheckResult(ctx context.Context, cr checkResult) error {
	f.results = append(f.results, cr)
	return nil
}

func failedResult(endpointID int, checkedAt time.Time, reasons ...string) checkResult {
	return checkResult{endpointID: endpointID, passed: false, reasons: reasons, checkedAt: checkedAt}
}

func TestRecordIncident(t *testing.T) {
	now := time.Now()
	errDB := errors.New("db down")

	tests := []struct {
		name        string
		store       *fakeStore
		result      checkResult
		wantReasons []string
		wantErr     error
	}{
		{
			name:        "first failure opens one",
			store:       &fakeStore{},
			result:      failedResult(3, now, "503 Service Unavailable"),
			wantReasons: []string{"503 Service Unavailable"},
		},
		{
			name:   "same reason already open is skipped",
			store:  &fakeStore{open: map[int][]incident{3: {{id: 1, endpointID: 3, reason: "503 Service Unavailable"}}}},
			result: failedResult(3, now, "503 Service Unavailable"),
		},
		{
			name:        "new reason opens a second incident",
			store:       &fakeStore{open: map[int][]incident{3: {{id: 1, endpointID: 3, reason: "404 Not Found"}}}},
			result:      failedResult(3, now, "503 Service Unavailable"),
			wantReasons: []string{"503 Service Unavailable"},
		},
		{
			name:        "several reasons open one incident each",
			store:       &fakeStore{},
			result:      failedResult(3, now, "status 503", "body missing"),
			wantReasons: []string{"status 503", "body missing"},
		},
		{
			name:        "only unknown reasons open",
			store:       &fakeStore{open: map[int][]incident{3: {{id: 1, endpointID: 3, reason: "status 503"}}}},
			result:      failedResult(3, now, "status 503", "body missing"),
			wantReasons: []string{"body missing"},
		},
		{
			name:        "repeated reason in one result opens once",
			store:       &fakeStore{},
			result:      failedResult(3, now, "slow", "slow"),
			wantReasons: []string{"slow"},
		},
		{
			name:        "open incident on other endpoint ignored",
			store:       &fakeStore{open: map[int][]incident{9: {{id: 1, endpointID: 9, reason: "503 Service Unavailable"}}}},
			result:      failedResult(3, now, "503 Service Unavailable"),
			wantReasons: []string{"503 Service Unavailable"},
		},
		{
			name:    "lookup error returned",
			store:   &fakeStore{openErr: errDB},
			result:  failedResult(3, now, "503 Service Unavailable"),
			wantErr: errDB,
		},
		{
			name:    "store error returned",
			store:   &fakeStore{storeErr: errDB},
			result:  failedResult(3, now, "503 Service Unavailable"),
			wantErr: errDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.store)

			opened, err := c.recordIncident(t.Context(), tt.result)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}

			var gotOpened, gotStored []string
			for _, inc := range opened {
				gotOpened = append(gotOpened, inc.reason)
				if inc.endpointID != 3 || !inc.startedAt.Equal(now) || !inc.resolvedAt.IsZero() {
					t.Errorf("incident = %+v", inc)
				}
			}
			for _, inc := range tt.store.incidents {
				gotStored = append(gotStored, inc.reason)
			}
			if !slices.Equal(gotOpened, tt.wantReasons) {
				t.Errorf("opened reasons = %q, want %q", gotOpened, tt.wantReasons)
			}
			if !slices.Equal(gotStored, tt.wantReasons) {
				t.Errorf("stored reasons = %q, want %q", gotStored, tt.wantReasons)
			}
		})
	}
}

func TestResolveIncidents(t *testing.T) {
	now := time.Now()
	errDB := errors.New("db down")
	passed := checkResult{endpointID: 3, passed: true, checkedAt: now}

	tests := []struct {
		name         string
		store        *fakeStore
		wantResolved []int // incident ids
		wantErr      error
	}{
		{
			name: "resolves every open incident",
			store: &fakeStore{open: map[int][]incident{3: {
				{id: 1, endpointID: 3, reason: "status 503"},
				{id: 2, endpointID: 3, reason: "body missing"},
			}}},
			wantResolved: []int{1, 2},
		},
		{name: "none open is a no-op", store: &fakeStore{}},
		{
			name:  "other endpoint untouched",
			store: &fakeStore{open: map[int][]incident{9: {{id: 1, endpointID: 9, reason: "status 503"}}}},
		},
		{name: "store error returned", store: &fakeStore{resolveErr: errDB}, wantErr: errDB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.store)

			resolved, err := c.resolveIncidents(t.Context(), passed)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			var got []int
			for _, inc := range resolved {
				got = append(got, inc.id)
				if !inc.resolvedAt.Equal(now) {
					t.Errorf("incident %d resolvedAt = %v, want %v", inc.id, inc.resolvedAt, now)
				}
			}
			if !slices.Equal(got, tt.wantResolved) {
				t.Errorf("resolved ids = %v, want %v", got, tt.wantResolved)
			}
		})
	}
}

// TestHandleResultLifecycle drives handleResult through the v1 lifecycle on a
// MemoryStore: open, no duplicate, second reason, resolve all, reopen.
func TestHandleResultLifecycle(t *testing.T) {
	ctx := t.Context()
	store := NewMemoryStore()
	c := NewChecker(store)
	t0 := time.Now()

	openReasons := func() []string {
		t.Helper()
		open, err := store.OpenIncidents(ctx, 3)
		if err != nil {
			t.Fatal(err)
		}
		var reasons []string
		for _, inc := range open {
			reasons = append(reasons, inc.reason)
		}
		return reasons
	}

	steps := []struct {
		name   string
		result checkResult
		want   []string
	}{
		{"first failure opens", failedResult(3, t0, "status 503"), []string{"status 503"}},
		{"same failure no duplicate", failedResult(3, t0.Add(time.Minute), "status 503"), []string{"status 503"}},
		{"new reason opens second", failedResult(3, t0.Add(2*time.Minute), "status 503", "body missing"), []string{"status 503", "body missing"}},
		{"pass resolves all", checkResult{endpointID: 3, passed: true, checkedAt: t0.Add(3 * time.Minute)}, nil},
		{"pass again is a no-op", checkResult{endpointID: 3, passed: true, checkedAt: t0.Add(4 * time.Minute)}, nil},
		{"failure after pass opens fresh", failedResult(3, t0.Add(5*time.Minute), "status 503"), []string{"status 503"}},
	}

	for _, s := range steps {
		c.handleResult(ctx, s.result)
		if got := openReasons(); !slices.Equal(got, s.want) {
			t.Fatalf("%s: open reasons = %q, want %q", s.name, got, s.want)
		}
	}
}
