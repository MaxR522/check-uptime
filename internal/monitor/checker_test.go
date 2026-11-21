package monitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCheckEndpoint(t *testing.T) {
	// 249 ASCII bytes then a 2-byte rune: the cut at 250 splits the rune.
	splitRuneBody := strings.Repeat("a", maxErrorBodyBytes-1) + "é" + "tail"

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		exps        []expectation
		timeout     time.Duration
		wantCode    int // 0 means no status code
		wantErr     bool
		wantErrBody string
	}{
		{
			name:     "ok",
			handler:  func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) },
			timeout:  time.Second,
			wantCode: http.StatusOK,
		},
		{
			name: "server error keeps body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("boom"))
			},
			timeout:     time.Second,
			wantCode:    http.StatusInternalServerError,
			wantErr:     true,
			wantErrBody: "boom",
		},
		{
			name: "error body truncated on rune boundary",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(splitRuneBody))
			},
			timeout:     time.Second,
			wantCode:    http.StatusServiceUnavailable,
			wantErr:     true,
			wantErrBody: strings.Repeat("a", maxErrorBodyBytes-1),
		},
		{
			name: "timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
				case <-time.After(2 * time.Second):
				}
			},
			timeout: 50 * time.Millisecond,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			ctx, cancel := context.WithTimeout(t.Context(), tt.timeout)
			defer cancel()

			got := checkEndpoint(ctx, srv.Client(), srv.URL, 7, mustAssertions(t, tt.exps))

			if got.endpointID != 7 {
				t.Errorf("endpointID = %d, want 7", got.endpointID)
			}
			if gotCode := deref(got.code, 0); gotCode != tt.wantCode {
				t.Errorf("code = %d, want %d", gotCode, tt.wantCode)
			}
			if (got.errMsg != nil) != tt.wantErr {
				t.Errorf("errMsg = %q, wantErr %v", deref(got.errMsg, ""), tt.wantErr)
			}
			gotBody := deref(got.errBody, "")
			if gotBody != tt.wantErrBody {
				t.Errorf("errBody = %q, want %q", gotBody, tt.wantErrBody)
			}
			if !utf8.ValidString(gotBody) {
				t.Errorf("errBody is not valid UTF-8: %q", gotBody)
			}
			if got.checkedAt.IsZero() {
				t.Error("checkedAt not set")
			}
		})
	}
}

func mustAssertions(t *testing.T, exps []expectation) []Assertion {
	t.Helper()
	a, err := buildAssertions(exps)
	if err != nil {
		t.Fatalf("buildAssertions: %v", err)
	}
	return a
}

func TestCheckEndpointAssertions(t *testing.T) {
	handler := func(code int, body string, delay time.Duration) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay)
			if code == http.StatusFound && r.URL.Path == "/" {
				http.Redirect(w, r, "/final", code)
				return
			}
			if code == http.StatusFound {
				code = http.StatusOK
			}
			w.WriteHeader(code)
			w.Write([]byte(body))
		}
	}
	exp := func(criteria, value string) expectation {
		return expectation{criteria: criteria, expectedValue: value}
	}

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		exps        []expectation
		wantPassed  bool
		wantReasons []string // substrings, one per expected reason
	}{
		{name: "no assertions, 200 passes", handler: handler(200, "ok", 0), wantPassed: true},
		{name: "no assertions, redirect followed passes", handler: handler(302, "", 0), wantPassed: true},
		{name: "no assertions, 500 fails by default", handler: handler(500, "", 0), wantReasons: []string{"500"}},
		{
			name:       "explicit status overrides default",
			handler:    handler(500, "", 0),
			exps:       []expectation{exp("status", "500")},
			wantPassed: true,
		},
		{
			name:        "status fails",
			handler:     handler(200, "", 0),
			exps:        []expectation{exp("status", "201")},
			wantReasons: []string{"status 200 not in 201"},
		},
		{
			name:        "max_response_time fails",
			handler:     handler(200, "", 50*time.Millisecond),
			exps:        []expectation{exp("max_response_time", "10ms")},
			wantReasons: []string{"response took"},
		},
		{
			name:        "body_contains fails",
			handler:     handler(200, "hello", 0),
			exps:        []expectation{exp("body_contains", "world")},
			wantReasons: []string{"body does not contain"},
		},
		{
			name:       "all assertions pass",
			handler:    handler(200, "hello world", 0),
			exps:       []expectation{exp("status", "2xx"), exp("max_response_time", "5s"), exp("body_contains", "world")},
			wantPassed: true,
		},
		{
			name:    "several fail together",
			handler: handler(200, "hello", 50*time.Millisecond),
			exps: []expectation{
				exp("status", "201"), exp("max_response_time", "10ms"), exp("body_contains", "world"),
			},
			wantReasons: []string{"status 200", "response took", "body does not contain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			got := checkEndpoint(t.Context(), srv.Client(), srv.URL, 1, mustAssertions(t, tt.exps))

			if got.passed != tt.wantPassed {
				t.Fatalf("passed = %v, want %v (reasons %q)", got.passed, tt.wantPassed, got.reasons)
			}
			if len(got.reasons) != len(tt.wantReasons) {
				t.Fatalf("reasons = %q, want %d", got.reasons, len(tt.wantReasons))
			}
			for i, want := range tt.wantReasons {
				if !strings.Contains(got.reasons[i], want) {
					t.Errorf("reasons[%d] = %q, want substring %q", i, got.reasons[i], want)
				}
			}
			if tt.wantPassed && (got.errMsg != nil || got.errBody != nil) {
				t.Errorf("passed check has errMsg %q errBody %q", deref(got.errMsg, ""), deref(got.errBody, ""))
			}
			if !tt.wantPassed && (got.errMsg == nil || got.errBody == nil) {
				t.Error("failed check missing errMsg or errBody")
			}
		})
	}
}

func TestBuildAssertions(t *testing.T) {
	tests := []struct {
		name    string
		exps    []expectation
		want    int
		wantErr bool
	}{
		{name: "none adds default status", want: 1},
		{name: "status replaces default", exps: []expectation{{criteria: "status", expectedValue: "2xx"}}, want: 1},
		{name: "other adds default", exps: []expectation{{criteria: "body_contains", expectedValue: "x"}}, want: 2},
		{name: "unknown criteria", exps: []expectation{{criteria: "nope", expectedValue: "x"}}, wantErr: true},
		{name: "bad value", exps: []expectation{{criteria: "max_response_time", expectedValue: "fast"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildAssertions(tt.exps)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestCheckEndpointConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	target := srv.URL
	srv.Close()

	got := checkEndpoint(t.Context(), http.DefaultClient, target, 1, mustAssertions(t, nil))

	if got.errMsg == nil || got.passed || len(got.reasons) != 1 {
		t.Fatalf("passed = %v, reasons = %q, want one connection error", got.passed, got.reasons)
	}
	if got.code != nil {
		t.Errorf("code = %d, want nil", *got.code)
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "no slashes", base: "https://example.com", path: "health", want: "https://example.com/health"},
		{name: "both slashes", base: "https://example.com/", path: "/health", want: "https://example.com/health"},
		{name: "empty path", base: "https://example.com", path: "", want: "https://example.com"},
		{name: "invalid base", base: "://bad", path: "health", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildURL(tt.base, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeStore records incidents and results; openErr makes OpenIncidents fail.
type fakeStore struct {
	open      map[int][]incident
	openErr   error
	incidents []incident
	results   []checkResult
}

func (f *fakeStore) GetWebsites(ctx context.Context) ([]website, error) { return nil, nil }

func (f *fakeStore) OpenIncidents(ctx context.Context, endpointID int) ([]incident, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	return f.open[endpointID], nil
}

func (f *fakeStore) StoreIncident(ctx context.Context, inc incident) error {
	f.incidents = append(f.incidents, inc)
	return nil
}

func (f *fakeStore) StoreCheckResult(ctx context.Context, cr checkResult) error {
	f.results = append(f.results, cr)
	return nil
}

func TestRecordIncident(t *testing.T) {
	failed := checkResult{endpointID: 3, passed: false, errMsg: new("503 Service Unavailable"), checkedAt: time.Now()}
	errDB := errors.New("db down")

	tests := []struct {
		name       string
		store      *fakeStore
		wantErr    error
		wantStored int
	}{
		{name: "no open incident opens one", store: &fakeStore{}, wantStored: 1},
		{
			name:       "same reason already open is skipped",
			store:      &fakeStore{open: map[int][]incident{3: {{id: 1, endpointID: 3, reason: "503 Service Unavailable"}}}},
			wantStored: 0,
		},
		{
			name:       "different reason opens another",
			store:      &fakeStore{open: map[int][]incident{3: {{id: 1, endpointID: 3, reason: "404 Not Found"}}}},
			wantStored: 1,
		},
		{
			name:       "open incident on other endpoint ignored",
			store:      &fakeStore{open: map[int][]incident{9: {{id: 1, endpointID: 9, reason: "503 Service Unavailable"}}}},
			wantStored: 1,
		},
		{name: "lookup error returned", store: &fakeStore{openErr: errDB}, wantErr: errDB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChecker(tt.store)

			err := c.recordIncident(t.Context(), failed)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if len(tt.store.incidents) != tt.wantStored {
				t.Fatalf("stored %d incidents, want %d", len(tt.store.incidents), tt.wantStored)
			}
			if tt.wantStored == 1 {
				got := tt.store.incidents[0]
				if got.endpointID != 3 || got.reason != "503 Service Unavailable" || !got.startedAt.Equal(failed.checkedAt) {
					t.Errorf("incident = %+v", got)
				}
			}
		})
	}
}

func TestCheckAllEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	valid := endpoint{id: 1, path: "ok", intervalCheckMin: 1, timeoutSec: 1, enabled: true}
	websites := []website{{
		baseURL: srv.URL,
		endpoints: []endpoint{
			valid,
			{id: 2, path: "disabled", intervalCheckMin: 1, timeoutSec: 1, enabled: false},
			{id: 3, path: "no-interval", intervalCheckMin: 0, timeoutSec: 1, enabled: true},
			{id: 4, path: "no-timeout", intervalCheckMin: 1, timeoutSec: 0, enabled: true},
			{id: 5, path: "bad-exp", intervalCheckMin: 1, timeoutSec: 1, enabled: true, expectations: []expectation{{criteria: "nope", expectedValue: "x"}}},
		},
	}}

	ctx, cancel := context.WithCancel(t.Context())
	c := NewChecker(&fakeStore{})
	c.client = srv.Client()
	results := c.checkAllEndpoints(ctx, websites)

	// Every endpoint is checked once immediately; only the valid one runs.
	select {
	case r := <-results:
		if r.endpointID != valid.id {
			t.Errorf("result for endpoint %d, want %d", r.endpointID, valid.id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no result for valid endpoint")
	}

	cancel()

	// The next tick is a minute away, so after cancel the channel must close
	// without another result.
	select {
	case r, ok := <-results:
		if ok {
			t.Fatalf("unexpected result for endpoint %d", r.endpointID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("results channel not closed after cancel")
	}
}
