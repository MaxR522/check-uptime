package monitor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxBodyBytes = 1 << 20
const maxErrorBodyBytes = 250

type Checker struct {
	store  Store
	client *http.Client
}

func NewChecker(store Store) *Checker {
	return &Checker{
		store: store,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

// RunChecker schedules every enabled endpoint and records results until ctx is done.
func (c *Checker) RunChecker(ctx context.Context) error {
	websites, err := c.store.GetWebsites(ctx)
	if err != nil {
		return fmt.Errorf("load websites: %w", err)
	}

	results := c.checkAllEndpoints(ctx, websites)

	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-results:
			if !ok {
				return nil
			}
			c.handleResult(ctx, r)
		}
	}
}

func (c *Checker) handleResult(ctx context.Context, r checkResult) {
	logger := slog.With("endpoint_id", r.endpointID, "duration_ms", r.durationMs)
	if r.code != nil {
		logger = logger.With("status", *r.code)
	}

	if !r.passed {
		logger.Warn("check failed", "err", deref(r.errMsg, ""))
		opened, err := c.recordIncident(ctx, r)
		if err != nil {
			logger.Error("record incident", "err", err)
		}
		for _, inc := range opened {
			logger.Info("incident opened", "err", inc.reason)
		}
	} else {
		logger.Info("check passed")
		resolved, err := c.resolveIncidents(ctx, r)
		if err != nil {
			logger.Error("resolve incidents", "err", err)
		}
		for _, inc := range resolved {
			logger.Info("incident resolved", "err", inc.reason)
		}
	}

	if err := c.store.StoreCheckResult(ctx, r); err != nil {
		logger.Error("store check result", "err", err)
	}
}

// recordIncident opens one incident per failure reason of a failed result,
// skipping reasons the endpoint already has an open incident for. It returns
// the incidents it opened, including those opened before an error.
func (c *Checker) recordIncident(ctx context.Context, r checkResult) ([]incident, error) {
	open, err := c.store.OpenIncidents(ctx, r.endpointID)
	if err != nil {
		return nil, fmt.Errorf("find open incidents: %w", err)
	}

	var opened []incident
	var errs []error
	for _, reason := range r.reasons {
		isKnown := func(inc incident) bool { return inc.reason == reason }
		if slices.ContainsFunc(open, isKnown) || slices.ContainsFunc(opened, isKnown) {
			continue
		}

		inc := incident{
			reason:     reason,
			startedAt:  r.checkedAt,
			endpointID: r.endpointID,
		}
		if err := c.store.StoreIncident(ctx, inc); err != nil {
			errs = append(errs, fmt.Errorf("store incident %q: %w", reason, err))
			continue
		}
		opened = append(opened, inc)
	}
	return opened, errors.Join(errs...)
}

// resolveIncidents resolves every open incident of the endpoint of a passing
// result and returns the ones it closed.
func (c *Checker) resolveIncidents(ctx context.Context, r checkResult) ([]incident, error) {
	resolved, err := c.store.ResolveIncidents(ctx, r.endpointID, r.checkedAt)
	if err != nil {
		return nil, fmt.Errorf("resolve incidents: %w", err)
	}
	return resolved, nil
}

// checkAllEndpoints starts one goroutine per valid enabled endpoint. The
// returned channel is closed once ctx is done and every goroutine has returned.
func (c *Checker) checkAllEndpoints(ctx context.Context, websites []website) <-chan checkResult {
	resultChan := make(chan checkResult)

	go func() {
		defer close(resultChan)
		var wg sync.WaitGroup

		for _, web := range websites {
			for _, ep := range web.endpoints {
				if !ep.enabled {
					continue
				}

				fullURL, err := buildURL(web.baseURL, ep.path)
				if err != nil {
					slog.Warn("skip endpoint", "endpoint_id", ep.id, "err", err)
					continue
				}

				if ep.intervalCheckMin <= 0 {
					slog.Warn("skip endpoint: invalid interval", "endpoint_id", ep.id, "url", fullURL, "interval_check_min", ep.intervalCheckMin)
					continue
				}

				if ep.timeoutSec <= 0 {
					slog.Warn("skip endpoint: invalid timeout", "endpoint_id", ep.id, "url", fullURL, "timeout_sec", ep.timeoutSec)
					continue
				}

				assertions, err := buildAssertions(ep.expectations)
				if err != nil {
					slog.Warn("skip endpoint: invalid expectations", "endpoint_id", ep.id, "url", fullURL, "err", err)
					continue
				}

				wg.Go(func() {
					ticker := time.NewTicker(time.Duration(ep.intervalCheckMin) * time.Minute)
					defer ticker.Stop()

					for {
						runCheckEndpoint(ctx, c.client, ep, fullURL, assertions, resultChan)

						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
						}
					}
				})
			}
		}

		wg.Wait()
	}()

	return resultChan
}

func buildURL(baseURL, path string) (string, error) {
	fullURL, err := url.JoinPath(baseURL, path)
	if err != nil {
		return "", fmt.Errorf("build url %q: %w", baseURL, err)
	}

	return fullURL, nil
}

func runCheckEndpoint(ctx context.Context, client *http.Client, ep endpoint, targetURL string, assertions []Assertion, resultChan chan<- checkResult) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(ep.timeoutSec*float64(time.Second)))
	result := checkEndpoint(ctxWithTimeout, client, targetURL, ep.id, assertions)
	cancel()

	select {
	case resultChan <- result:
	case <-ctx.Done():
	}
}

// checkEndpoint performs one GET request and runs every assertion against the
// response. The check fails on a request error or any failing assertion; all
// failure reasons are recorded. durationMs covers the request and the body
// read, set by the deferred func on every return path.
func checkEndpoint(ctx context.Context, client *http.Client, target string, endpointID int, assertions []Assertion) (result checkResult) {
	start := time.Now()
	result = checkResult{checkedAt: start, endpointID: endpointID}
	defer func() {
		result.durationMs = time.Since(start).Milliseconds()
		result.passed = len(result.reasons) == 0
		if !result.passed {
			result.errMsg = new(strings.Join(result.reasons, "; "))
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		result.reasons = []string{err.Error()}
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.reasons = []string{err.Error()}
		return result
	}
	defer resp.Body.Close()

	result.code = new(resp.StatusCode)

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		result.reasons = []string{err.Error()}
		return result
	}

	elapsed := time.Since(start)
	for _, a := range assertions {
		if err := a.Check(resp, body, elapsed); err != nil {
			result.reasons = append(result.reasons, err.Error())
		}
	}
	if len(result.reasons) > 0 {
		result.errBody = new(truncateUTF8(body, maxErrorBodyBytes))
	}
	return result
}

// truncateUTF8 returns at most n bytes of b as a string, dropping a rune
// split by the cut.
func truncateUTF8(b []byte, n int) string {
	return strings.ToValidUTF8(string(b[:min(len(b), n)]), "")
}
