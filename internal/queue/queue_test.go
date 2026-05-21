package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// recv reads one value from ch, failing the test if nothing arrives in time.
func recv[T any](t *testing.T, ch <-chan T) (T, bool) {
	t.Helper()
	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(2 * time.Second):
		var zero T
		t.Fatal("timed out waiting on channel")
		return zero, false
	}
}

func noop(context.Context, func(tools.Progress)) {}

func TestEnqueueSubscribeComplete(t *testing.T) {
	q := New()
	id := q.Enqueue("image", "convert", func(_ context.Context, emit func(tools.Progress)) {
		emit(tools.Progress{Message: "one"})
		emit(tools.Progress{Message: "two", Completed: true})
	})

	ch, err := q.Subscribe(context.Background(), id)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	var msgs []string
	for p := range ch {
		msgs = append(msgs, p.Message)
		if p.JobID != id {
			t.Errorf("JobID = %q, want %q", p.JobID, id)
		}
		if p.Tool != "image" || p.Action != "convert" {
			t.Errorf("Tool/Action = %q/%q, want image/convert", p.Tool, p.Action)
		}
	}
	if len(msgs) != 2 || msgs[0] != "one" || msgs[1] != "two" {
		t.Fatalf("messages = %v, want [one two]", msgs)
	}

	j := q.Get(id)
	if j == nil || !j.Completed {
		t.Fatalf("job not completed: %+v", j)
	}
}

func TestSubscribeReplaysAfterCompletion(t *testing.T) {
	q := New()
	id := q.Enqueue("pdf", "merge", func(_ context.Context, emit func(tools.Progress)) {
		emit(tools.Progress{Message: "a"})
		emit(tools.Progress{Message: "b"})
		emit(tools.Progress{Message: "c"})
	})

	// Drain once so the job is definitely complete.
	ch1, _ := q.Subscribe(context.Background(), id)
	for range ch1 {
	}

	// A fresh subscriber must replay the full history, then close.
	ch2, _ := q.Subscribe(context.Background(), id)
	n := 0
	for range ch2 {
		n++
	}
	if n != 3 {
		t.Fatalf("replay count = %d, want 3", n)
	}
}

func TestSubscribeLive(t *testing.T) {
	q := New()
	step := make(chan struct{})
	id := q.Enqueue("archive", "extract", func(_ context.Context, emit func(tools.Progress)) {
		<-step
		emit(tools.Progress{Message: "live"})
		<-step
	})

	ch, err := q.Subscribe(context.Background(), id)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	step <- struct{}{} // unblock the emit
	p, ok := recv(t, ch)
	if !ok || p.Message != "live" {
		t.Fatalf("got (%q, %v), want (live, true)", p.Message, ok)
	}

	step <- struct{}{} // let the runner return
	if _, ok := recv(t, ch); ok {
		t.Fatal("channel still open after completion")
	}
}

func TestSubscribeAllLifecycle(t *testing.T) {
	q := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	all := q.SubscribeAll(ctx)
	id := q.Enqueue("image", "convert", func(_ context.Context, emit func(tools.Progress)) {
		emit(tools.Progress{Message: "x"})
	})

	deadline := time.After(2 * time.Second)
	sawPending, sawComplete := false, false
	for !sawComplete {
		select {
		case j := <-all:
			if j.ID != id {
				continue
			}
			if j.Completed {
				sawComplete = true
			} else {
				sawPending = true
			}
		case <-deadline:
			t.Fatal("timed out waiting for lifecycle events")
		}
	}
	if !sawPending {
		t.Error("never saw a pre-completion snapshot")
	}
}

func TestCancel(t *testing.T) {
	q := New()
	started := make(chan struct{})
	id := q.Enqueue("archive", "extract", func(ctx context.Context, emit func(tools.Progress)) {
		emit(tools.Progress{Message: "started"})
		close(started)
		<-ctx.Done()
		emit(tools.Progress{Message: "aborted", Err: &tools.Error{Code: tools.CodeAborted}})
	})

	ch, _ := q.Subscribe(context.Background(), id)
	if first, _ := recv(t, ch); first.Message != "started" {
		t.Fatalf("first message = %q, want started", first.Message)
	}
	<-started

	if err := q.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	var last tools.Progress
	for p := range ch {
		last = p
	}
	if last.Message != "aborted" {
		t.Fatalf("last message = %q, want aborted", last.Message)
	}

	j := q.Get(id)
	if j == nil || j.Err == nil || j.Err.Code != tools.CodeAborted {
		t.Fatalf("job Err = %v, want ABORTED", j.Err)
	}
}

func TestUnknownJob(t *testing.T) {
	q := New()
	if _, err := q.Subscribe(context.Background(), "nope"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("Subscribe err = %v, want ErrJobNotFound", err)
	}
	if err := q.Cancel("nope"); !errors.Is(err, ErrJobNotFound) {
		t.Errorf("Cancel err = %v, want ErrJobNotFound", err)
	}
	if j := q.Get("nope"); j != nil {
		t.Errorf("Get = %+v, want nil", j)
	}
}

func TestListOrder(t *testing.T) {
	q := New()
	id1 := q.Enqueue("image", "convert", noop)
	id2 := q.Enqueue("pdf", "split", noop)
	id3 := q.Enqueue("archive", "compress", noop)

	got := q.List()
	if len(got) != 3 {
		t.Fatalf("List len = %d, want 3", len(got))
	}
	for i, want := range []string{id1, id2, id3} {
		if got[i].ID != want {
			t.Errorf("List[%d].ID = %q, want %q", i, got[i].ID, want)
		}
	}
}
