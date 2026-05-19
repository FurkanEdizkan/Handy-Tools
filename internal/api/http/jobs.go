package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// jobs is the phase-1 in-memory job tracker. Each job buffers every Progress
// event it has ever emitted plus a "done" flag, so a late SSE subscriber can
// replay from the beginning instead of missing already-completed events.
//
// Phase 3 of the pivot replaces this with internal/queue/, which has the same
// shape but is shared across transports (TUI + gRPC + HTTP) and gains lifecycle
// management. Keep this file small — it exists to unblock the HTTP transport.
type jobs struct {
	mu sync.Mutex
	m  map[string]*job
}

// job holds the event history and the wait-queue of subscribers blocked on
// the next append. Subscribers register a notify channel under the mutex; the
// producer closes those channels on each append or on completion to wake them.
type job struct {
	mu      sync.Mutex
	events  []tools.Progress
	done    bool
	waiters []chan struct{}
}

func newJobs() *jobs { return &jobs{m: map[string]*job{}} }

// create allocates a job ID and registers it. The caller starts the work in a
// goroutine and uses (*jobs).append / .complete to publish events.
func (js *jobs) create() (string, *job) {
	id := randomID()
	j := &job{}
	js.mu.Lock()
	js.m[id] = j
	js.mu.Unlock()
	return id, j
}

func (js *jobs) get(id string) *job {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.m[id]
}

// append publishes p to subscribers and persists it for late joiners.
func (j *job) append(p tools.Progress) {
	j.mu.Lock()
	j.events = append(j.events, p)
	waiters := j.waiters
	j.waiters = nil
	j.mu.Unlock()
	for _, w := range waiters {
		close(w)
	}
}

// complete marks the job as finished. Subscribers will drain the remaining
// history and then return cleanly.
func (j *job) complete() {
	j.mu.Lock()
	j.done = true
	waiters := j.waiters
	j.waiters = nil
	j.mu.Unlock()
	for _, w := range waiters {
		close(w)
	}
}

// stream drains events to write in order, blocking when caught up until either
// the next append, completion, or ctx cancellation. It is safe to call
// concurrently from multiple subscribers — each one tracks its own cursor.
func (j *job) stream(ctx context.Context, write func(tools.Progress) error) error {
	idx := 0
	for {
		j.mu.Lock()
		for idx < len(j.events) {
			p := j.events[idx]
			idx++
			j.mu.Unlock()
			if err := write(p); err != nil {
				return err
			}
			j.mu.Lock()
		}
		if j.done {
			j.mu.Unlock()
			return nil
		}
		ch := make(chan struct{})
		j.waiters = append(j.waiters, ch)
		j.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand on a sane OS does not fail; if it does, the server is
		// in such a degraded state that a fixed string is acceptable.
		return "00000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}
