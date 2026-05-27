// Package queue is the shared in-memory job registry for Handy Tools.
//
// It is the single source of truth for jobs and progress fan-out: the TUI,
// the HTTP transport, and the gRPC server all enqueue work here and subscribe
// to the same events, instead of each maintaining its own ad-hoc tracker.
//
// A job is created with Enqueue, which takes a Runner — the bridge to the
// tool layer. The Runner receives a cancelable context and an emit callback;
// every tools.Progress it emits is buffered (so late subscribers can replay
// the full history) and fanned out to live subscribers.
//
// There is no persistence — jobs live only for the process lifetime. Optional
// disk persistence is a later phase.
package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// ErrJobNotFound is returned by Subscribe and Cancel for an unknown job ID.
var ErrJobNotFound = errors.New("queue: job not found")

// Job is an immutable snapshot of a queued job's state. Callers receive Job
// by value from List, Get, and SubscribeAll; mutating it does not affect the
// queue.
type Job struct {
	ID        string
	Tool      string // "image" | "archive" | "pdf"
	Action    string // "convert" | "extract" | "merge" | ...
	StartedAt time.Time
	Completed bool           // the Runner has returned
	Err       *tools.Error   // last error reported via Progress, if any
	Progress  tools.Progress // most recent progress snapshot
}

// Runner performs the actual work for a job. It runs in its own goroutine.
// It must publish progress through emit and should return promptly when ctx
// is canceled (cooperative cancellation — see Queue.Cancel).
//
// Note: the proposed API in issue #74 omitted ctx; it is included here
// because the Runner cannot honor cancellation without it.
type Runner func(ctx context.Context, emit func(tools.Progress))

// jobEntry is the internal mutable state for one job.
type jobEntry struct {
	cancel context.CancelFunc

	mu      sync.Mutex
	meta    Job
	events  []tools.Progress // full history, for late subscribers
	done    bool
	waiters []chan struct{} // per-job subscribers parked on the next append
}

func (e *jobEntry) snapshot() Job {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.meta
}

// append records p, refreshes the snapshot, and wakes parked subscribers.
func (e *jobEntry) append(p tools.Progress) {
	e.mu.Lock()
	e.events = append(e.events, p)
	e.meta.Progress = p
	if p.Err != nil {
		e.meta.Err = p.Err
	}
	waiters := e.waiters
	e.waiters = nil
	e.mu.Unlock()
	for _, w := range waiters {
		close(w)
	}
}

// complete marks the job finished; subscribers drain history then return.
func (e *jobEntry) complete() {
	e.mu.Lock()
	e.done = true
	e.meta.Completed = true
	waiters := e.waiters
	e.waiters = nil
	e.mu.Unlock()
	for _, w := range waiters {
		close(w)
	}
}

// stream drains buffered events to write in order, blocking when caught up
// until the next append, completion, or ctx cancellation. write returns false
// to stop early. Safe for concurrent callers — each tracks its own cursor.
func (e *jobEntry) stream(ctx context.Context, write func(tools.Progress) bool) {
	idx := 0
	for {
		e.mu.Lock()
		for idx < len(e.events) {
			p := e.events[idx]
			idx++
			e.mu.Unlock()
			if !write(p) {
				return
			}
			e.mu.Lock()
		}
		if e.done {
			e.mu.Unlock()
			return
		}
		ch := make(chan struct{})
		e.waiters = append(e.waiters, ch)
		e.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return
		}
	}
}

// Queue is the shared job registry. The zero value is not usable; call New.
type Queue struct {
	mu      sync.Mutex
	order   []string // job IDs, oldest first
	jobs    map[string]*jobEntry
	allSubs map[chan Job]struct{}
}

// New returns an empty Queue.
func New() *Queue {
	return &Queue{
		jobs:    make(map[string]*jobEntry),
		allSubs: make(map[chan Job]struct{}),
	}
}

// Enqueue registers a new job and starts runner in a goroutine. It returns
// the job ID immediately. The job completes when runner returns.
func (q *Queue) Enqueue(tool, action string, runner Runner) (jobID string) {
	id := randomID()
	ctx, cancel := context.WithCancel(context.Background())
	e := &jobEntry{
		cancel: cancel,
		meta: Job{
			ID:        id,
			Tool:      tool,
			Action:    action,
			StartedAt: time.Now(),
		},
	}

	q.mu.Lock()
	q.jobs[id] = e
	q.order = append(q.order, id)
	q.mu.Unlock()

	q.broadcast(e.snapshot()) // job-start lifecycle event

	go func() {
		defer cancel() // release the context once the runner is done
		emit := func(p tools.Progress) {
			p.JobID = id
			if p.Tool == "" {
				p.Tool = tool
			}
			if p.Action == "" {
				p.Action = action
			}
			e.append(p)
			q.broadcast(e.snapshot())
		}
		runner(ctx, emit)
		e.complete()
		q.broadcast(e.snapshot())
	}()

	return id
}

// broadcast fans a job snapshot out to every SubscribeAll listener. A slow
// listener's send is dropped rather than blocking the producer — every Job is
// a full snapshot, so the listener stays eventually-consistent.
func (q *Queue) broadcast(j Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for ch := range q.allSubs {
		select {
		case ch <- j:
		default:
		}
	}
}

// Subscribe streams every progress event for one job — replaying history from
// the start, then live events — until the job completes or ctx is canceled.
// The returned channel is closed when the stream ends.
func (q *Queue) Subscribe(ctx context.Context, jobID string) (<-chan tools.Progress, error) {
	q.mu.Lock()
	e := q.jobs[jobID]
	q.mu.Unlock()
	if e == nil {
		return nil, ErrJobNotFound
	}
	ch := make(chan tools.Progress)
	go func() {
		defer close(ch)
		e.stream(ctx, func(p tools.Progress) bool {
			select {
			case ch <- p:
				return true
			case <-ctx.Done():
				return false
			}
		})
	}()
	return ch, nil
}

// SubscribeAllCount returns the number of currently-registered SubscribeAll
// listeners. Exists for tests that need to know when a handler has finished
// registering before enqueuing a job — SubscribeAll is live-only, so a
// pre-subscription Enqueue's snapshot is lost.
func (q *Queue) SubscribeAllCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.allSubs)
}

// SubscribeAll streams Job lifecycle snapshots (start, each progress update,
// completion) across all jobs. The channel is closed when ctx is canceled.
func (q *Queue) SubscribeAll(ctx context.Context) <-chan Job {
	ch := make(chan Job, 64)
	q.mu.Lock()
	q.allSubs[ch] = struct{}{}
	q.mu.Unlock()
	go func() {
		<-ctx.Done()
		q.mu.Lock()
		if _, ok := q.allSubs[ch]; ok {
			delete(q.allSubs, ch)
			close(ch)
		}
		q.mu.Unlock()
	}()
	return ch
}

// List returns a snapshot of all known jobs, oldest first.
func (q *Queue) List() []Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Job, 0, len(q.order))
	for _, id := range q.order {
		out = append(out, q.jobs[id].snapshot())
	}
	return out
}

// Get returns a snapshot of one job, or nil if the ID is unknown.
func (q *Queue) Get(jobID string) *Job {
	q.mu.Lock()
	e := q.jobs[jobID]
	q.mu.Unlock()
	if e == nil {
		return nil
	}
	j := e.snapshot()
	return &j
}

// Cancel cooperatively cancels a job by canceling its context. The Runner is
// responsible for observing ctx and returning. Canceling an already-finished
// job is a no-op. Returns ErrJobNotFound for an unknown ID.
func (q *Queue) Cancel(jobID string) error {
	q.mu.Lock()
	e := q.jobs[jobID]
	q.mu.Unlock()
	if e == nil {
		return ErrJobNotFound
	}
	e.cancel()
	return nil
}

// randomID returns a 24-hex-char job ID. crypto/rand on a sane OS does not
// fail; a fixed fallback keeps the queue usable in the degenerate case.
func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "000000000000000000000000"
	}
	return hex.EncodeToString(b[:])
}
