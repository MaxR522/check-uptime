package monitor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// job is an endpoint validated and ready to run: resolved URL and parsed
// assertions.
type job struct {
	ep         endpoint
	url        string
	assertions []Assertion
}

// prepareJob validates an endpoint of web and builds the job to schedule. A
// disabled endpoint or an invalid interval, timeout, URL or expectation is an
// error.
func prepareJob(web website, ep endpoint) (job, error) {
	if !ep.enabled {
		return job{}, errors.New("endpoint disabled")
	}

	fullURL, err := buildURL(web.baseURL, ep.path)
	if err != nil {
		return job{}, err
	}
	if ep.intervalCheckMin <= 0 {
		return job{}, fmt.Errorf("invalid interval %d min", ep.intervalCheckMin)
	}
	if ep.timeoutSec <= 0 {
		return job{}, fmt.Errorf("invalid timeout %v sec", ep.timeoutSec)
	}

	assertions, err := buildAssertions(ep.expectations)
	if err != nil {
		return job{}, fmt.Errorf("invalid expectations: %w", err)
	}

	return job{ep: ep, url: fullURL, assertions: assertions}, nil
}

// Scheduler runs one check loop per endpoint and lets callers start, restart
// and stop each one at runtime. Results of all loops arrive on Results. The
// channel is never closed: loops stop on the parent context and receivers
// should do the same.
type Scheduler struct {
	client  *http.Client
	unit    time.Duration // length of one interval step; time.Minute, shortened in tests
	ctx     context.Context
	results chan checkResult

	mu      sync.Mutex
	cancels map[int]context.CancelFunc
	wg      sync.WaitGroup
}

// NewScheduler returns a scheduler whose loops stop when ctx is done.
func NewScheduler(ctx context.Context, client *http.Client) *Scheduler {
	return &Scheduler{
		client:  client,
		unit:    time.Minute,
		ctx:     ctx,
		results: make(chan checkResult),
		cancels: make(map[int]context.CancelFunc),
	}
}

// Results returns the channel every check result is sent on.
func (s *Scheduler) Results() <-chan checkResult {
	return s.results
}

// Wait blocks until the parent context is done and every loop has returned.
func (s *Scheduler) Wait() {
	<-s.ctx.Done()

	// Holding mu orders any Start that passed its ctx check before wg.Wait,
	// and makes later Starts and Stops wait; Starts then see the cancelled ctx
	// and return. Loops never take mu, so this cannot deadlock.
	s.mu.Lock()
	defer s.mu.Unlock()

	s.wg.Wait()
}

// Start schedules j: first check immediately, then every interval. An endpoint
// that is already scheduled is replaced. It does nothing once the parent
// context is done.
func (s *Scheduler) Start(j job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ctx.Err() != nil {
		return
	}
	s.stopLocked(j.ep.id)

	ctx, cancel := context.WithCancel(s.ctx)
	s.cancels[j.ep.id] = cancel

	s.wg.Go(func() {
		ticker := time.NewTicker(time.Duration(j.ep.intervalCheckMin) * s.unit)
		defer ticker.Stop()

		for {
			runCheckEndpoint(ctx, s.client, j.ep, j.url, j.assertions, s.results)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

// Stop cancels the schedule of the endpoint. An unknown id is a no-op.
func (s *Scheduler) Stop(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked(id)
}

// Restart replaces the schedule of j's endpoint, starting it if absent.
func (s *Scheduler) Restart(j job) {
	s.Stop(j.ep.id)
	s.Start(j)
}

func (s *Scheduler) stopLocked(id int) {
	if cancel, ok := s.cancels[id]; ok {
		cancel()
		delete(s.cancels, id)
	}
}
