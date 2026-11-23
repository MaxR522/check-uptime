package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// hitCounter counts requests per path.
type hitCounter struct {
	mu   sync.Mutex
	hits map[string]int
}

func (h *hitCounter) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hits[r.URL.Path]++
}

func (h *hitCounter) count(path string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits[path]
}

// testScheduler returns a scheduler with a 10ms interval unit that drains its
// results, a counting server, and a job builder for it.
func testScheduler(t *testing.T) (*Scheduler, *hitCounter, func(id int, path string) job, context.CancelFunc) {
	t.Helper()

	h := &hitCounter{hits: map[string]int{}}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	s := NewScheduler(ctx, srv.Client())
	s.unit = 10 * time.Millisecond

	go func() {
		for {
			select {
			case <-s.Results():
			case <-ctx.Done():
				return
			}
		}
	}()

	mk := func(id int, path string) job {
		ep := endpoint{id: id, path: path, intervalCheckMin: 1, timeoutSec: 1, enabled: true}
		j, err := prepareJob(website{baseURL: srv.URL}, ep)
		if err != nil {
			t.Fatalf("prepareJob: %v", err)
		}
		return j
	}
	return s, h, mk, cancel
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestSchedulerStartRepeats(t *testing.T) {
	s, h, mk, cancel := testScheduler(t)
	defer cancel()

	s.Start(mk(1, "/a"))
	waitFor(t, "three checks", func() bool { return h.count("/a") >= 3 })
}

func TestSchedulerStop(t *testing.T) {
	s, h, mk, cancel := testScheduler(t)
	defer cancel()

	s.Start(mk(1, "/a"))
	waitFor(t, "first check", func() bool { return h.count("/a") >= 1 })

	s.Stop(1)
	time.Sleep(30 * time.Millisecond) // let an in-flight check finish
	before := h.count("/a")
	time.Sleep(100 * time.Millisecond)
	if after := h.count("/a"); after != before {
		t.Errorf("hits grew after Stop: %d -> %d", before, after)
	}

	s.Stop(1)  // already stopped
	s.Stop(99) // unknown
}

func TestSchedulerRestart(t *testing.T) {
	s, h, mk, cancel := testScheduler(t)
	defer cancel()

	s.Start(mk(1, "/old"))
	waitFor(t, "old check", func() bool { return h.count("/old") >= 1 })

	s.Restart(mk(1, "/new"))
	waitFor(t, "new check", func() bool { return h.count("/new") >= 2 })

	time.Sleep(30 * time.Millisecond)
	before := h.count("/old")
	time.Sleep(100 * time.Millisecond)
	if after := h.count("/old"); after != before {
		t.Errorf("old schedule still running: %d -> %d", before, after)
	}
}

func TestSchedulerRestartAbsentStarts(t *testing.T) {
	s, h, mk, cancel := testScheduler(t)
	defer cancel()

	s.Restart(mk(1, "/a"))
	waitFor(t, "check", func() bool { return h.count("/a") >= 1 })
}

func TestSchedulerStartSameIDKeepsOneSchedule(t *testing.T) {
	s, h, mk, cancel := testScheduler(t)
	defer cancel()

	s.Start(mk(1, "/a"))
	s.Start(mk(1, "/a"))
	s.Start(mk(1, "/a"))

	time.Sleep(200 * time.Millisecond)
	cancel()
	s.Wait()

	// One loop at 10ms ticks gives about 20 hits plus the immediate ones; three
	// loops would give about 60. Allow generous slack for slow machines.
	if got := h.count("/a"); got > 40 {
		t.Errorf("hits = %d, want a single schedule (<= 40)", got)
	}
}

func TestSchedulerShutdown(t *testing.T) {
	s, h, mk, cancel := testScheduler(t)

	s.Start(mk(1, "/a"))
	s.Start(mk(2, "/b"))
	waitFor(t, "checks", func() bool { return h.count("/a") >= 1 && h.count("/b") >= 1 })

	cancel()

	waited := make(chan struct{})
	go func() {
		s.Wait()
		close(waited)
	}()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return after cancel")
	}

	before := h.count("/a") + h.count("/b")
	s.Start(mk(3, "/c")) // no-op after shutdown
	time.Sleep(50 * time.Millisecond)
	if h.count("/c") != 0 {
		t.Error("Start after shutdown ran a check")
	}
	if after := h.count("/a") + h.count("/b"); after != before {
		t.Errorf("hits grew after shutdown: %d -> %d", before, after)
	}
}
