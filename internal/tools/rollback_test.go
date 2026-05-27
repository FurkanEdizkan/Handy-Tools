package tools

import (
	"errors"
	"testing"
)

func TestRollbackStackReplaysInReverse(t *testing.T) {
	var order []string
	var stack RollbackStack
	for _, label := range []string{"a", "b", "c"} {
		l := label
		stack.Push(RollbackStep{
			Undo: func() error { order = append(order, l); return nil },
			Note: l,
		})
	}
	failures := stack.Replay()
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %+v", failures)
	}
	want := []string{"c", "b", "a"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestRollbackStackCollectsFailuresButKeepsGoing(t *testing.T) {
	var stack RollbackStack
	stack.Push(RollbackStep{Undo: func() error { return errors.New("step1 boom") }, Note: "step1"})
	stack.Push(RollbackStep{Undo: func() error { return nil }, Note: "step2"})
	stack.Push(RollbackStep{Undo: func() error { return errors.New("step3 boom") }, Note: "step3"})

	failures := stack.Replay()
	if len(failures) != 2 {
		t.Fatalf("want 2 rollback failures, got %d: %+v", len(failures), failures)
	}
	// Replays in reverse: step3 (fails), step2 (succeeds), step1 (fails).
	if failures[0].Path != "step3" || failures[1].Path != "step1" {
		t.Errorf("failure order = [%q, %q], want [step3, step1]", failures[0].Path, failures[1].Path)
	}
	for _, f := range failures {
		if f.Code != CodeRollbackFailed {
			t.Errorf("failure code = %q, want %q", f.Code, CodeRollbackFailed)
		}
	}
	if stack.Len() != 0 {
		t.Errorf("Replay should empty the stack, len = %d", stack.Len())
	}
}

func TestCoalesceFailureCode(t *testing.T) {
	cases := []struct {
		name string
		in   []Failure
		want string
	}{
		{"empty", nil, CodeIO},
		{"unanimous", []Failure{{Code: CodePermissionDenied}, {Code: CodePermissionDenied}}, CodePermissionDenied},
		{"mixed", []Failure{{Code: CodePermissionDenied}, {Code: CodeNotFound}}, CodeIO},
		{"single", []Failure{{Code: CodeNotFound}}, CodeNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoalesceFailureCode(tc.in); got != tc.want {
				t.Errorf("CoalesceFailureCode(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
