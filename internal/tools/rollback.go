package tools

// RollbackStep is one reversible action accumulated during a batch op.
// Undo runs the reversal; Note is a short human-readable label used when
// emitting per-step rollback progress events.
type RollbackStep struct {
	Undo func() error
	Note string
}

// RollbackStack records reversible steps as a batch progresses and can
// replay them in reverse on failure. Use the typical pattern:
//
//	var rb tools.RollbackStack
//	for each item:
//	    if err := doMutation(item); err != nil {
//	        failures := rb.Replay()  // best-effort
//	        // surface failures + abort
//	    }
//	    rb.Push(tools.RollbackStep{Undo: func() error { return undoMutation(item) }, Note: ...})
//
// Replay never panics: every step is invoked, and any error becomes a
// Failure entry tagged ROLLBACK_FAILED in the returned slice (with Note
// in the Message field for diagnostics).
type RollbackStack struct {
	steps []RollbackStep
}

// Push appends a step. Order of pushes is the order of mutations; Replay
// walks them in reverse.
func (s *RollbackStack) Push(step RollbackStep) {
	s.steps = append(s.steps, step)
}

// Replay invokes every step's Undo in reverse order, regardless of whether
// earlier reversals succeeded. Returns one Failure per step that errored.
// The stack is consumed (steps are cleared) so a partial-success replay
// can't be re-run by accident.
func (s *RollbackStack) Replay() []Failure {
	var out []Failure
	for i := len(s.steps) - 1; i >= 0; i-- {
		step := s.steps[i]
		if step.Undo == nil {
			continue
		}
		if err := step.Undo(); err != nil {
			out = append(out, Failure{
				Path:    step.Note,
				Code:    CodeRollbackFailed,
				Message: err.Error(),
			})
		}
	}
	s.steps = nil
	return out
}

// Len returns the number of pending undo steps.
func (s *RollbackStack) Len() int { return len(s.steps) }
