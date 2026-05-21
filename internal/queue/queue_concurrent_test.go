package queue

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// drain reads ch to completion and returns how many events arrived.
func drain(ch <-chan tools.Progress) int {
	n := 0
	for range ch {
		n++
	}
	return n
}

// TestMultipleSubscribersSameJob confirms two concurrent Subscribe calls on the
// same job each see the full event stream — neither steals events from the
// other (every subscriber tracks its own cursor).
func TestMultipleSubscribersSameJob(t *testing.T) {
	t.Parallel()
	q := New()
	const events = 50
	id := q.Enqueue("image", "convert", func(_ context.Context, emit func(tools.Progress)) {
		for i := 0; i < events; i++ {
			emit(tools.Progress{Message: fmt.Sprintf("e%d", i)})
		}
		emit(tools.Progress{Message: "done", Completed: true})
	})

	counts := make([]int, 2)
	var wg sync.WaitGroup
	for i := range counts {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			ch, err := q.Subscribe(context.Background(), id)
			if err != nil {
				t.Errorf("Subscribe: %v", err)
				return
			}
			counts[slot] = drain(ch)
		}(i)
	}
	wg.Wait()

	for i, got := range counts {
		if got != events+1 {
			t.Errorf("subscriber %d saw %d events, want %d", i, got, events+1)
		}
	}
}

// TestSubscriberDisconnectIsolation confirms that canceling one subscriber's
// context closes only that subscriber's channel — the other keeps streaming
// through to completion.
func TestSubscriberDisconnectIsolation(t *testing.T) {
	t.Parallel()
	q := New()
	release := make(chan struct{})
	id := q.Enqueue("archive", "extract", func(_ context.Context, emit func(tools.Progress)) {
		emit(tools.Progress{Message: "first"})
		<-release
		emit(tools.Progress{Message: "second", Completed: true})
	})

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	chA, err := q.Subscribe(ctxA, id)
	if err != nil {
		t.Fatalf("Subscribe A: %v", err)
	}
	chB, err := q.Subscribe(context.Background(), id)
	if err != nil {
		t.Fatalf("Subscribe B: %v", err)
	}

	// Both subscribers replay the first event.
	if p, _ := recv(t, chA); p.Message != "first" {
		t.Fatalf("A first event = %q, want first", p.Message)
	}
	if p, _ := recv(t, chB); p.Message != "first" {
		t.Fatalf("B first event = %q, want first", p.Message)
	}

	// Disconnect A. Its channel must close; B is unaffected.
	cancelA()
	if drained := drain(chA); drained > 1 {
		t.Errorf("A drained %d extra events after cancel, want 0..1", drained)
	}

	// B still receives the remaining event and the completion close.
	close(release)
	var last string
	for p := range chB {
		last = p.Message
	}
	if last != "second" {
		t.Fatalf("B last event = %q, want second", last)
	}

	if j := q.Get(id); j == nil || !j.Completed {
		t.Fatalf("job not completed after B drained: %+v", j)
	}
}

// TestStressEnqueueSubscribe hammers the queue with 1000 jobs and 5 subscribers
// each. Run under -race it shotguns the append/stream/broadcast paths for data
// races; every subscriber must still see exactly its job's event count.
func TestStressEnqueueSubscribe(t *testing.T) {
	t.Parallel()
	q := New()
	const (
		jobs         = 1000
		subsPerJob   = 5
		eventsPerJob = 3
	)

	var wg sync.WaitGroup
	for j := 0; j < jobs; j++ {
		id := q.Enqueue("image", "convert", func(_ context.Context, emit func(tools.Progress)) {
			for k := 0; k < eventsPerJob; k++ {
				emit(tools.Progress{Message: "tick"})
			}
		})
		for s := 0; s < subsPerJob; s++ {
			wg.Add(1)
			go func(jobID string) {
				defer wg.Done()
				ch, err := q.Subscribe(context.Background(), jobID)
				if err != nil {
					t.Errorf("Subscribe(%s): %v", jobID, err)
					return
				}
				if got := drain(ch); got != eventsPerJob {
					t.Errorf("job %s subscriber saw %d events, want %d", jobID, got, eventsPerJob)
				}
			}(id)
		}
	}
	wg.Wait()

	if got := len(q.List()); got != jobs {
		t.Fatalf("List len = %d, want %d", got, jobs)
	}
}
