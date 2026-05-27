// Package rename performs batch file renaming driven by a Go regexp on each
// file's basename plus a replacement template. Pure Go — stdlib regexp and
// path/filepath only. The pipeline mirrors the rest of internal/tools/: a
// Request struct describes the job and Run returns a <-chan tools.Progress
// emitting one event per processed file plus a terminal summary.
//
// Inspect is the dry-run / preflight: it compiles the pattern, walks the
// source list, and returns the rename plan synchronously without touching
// the filesystem. Both the CLI's --dry-run and the (future) UI's preview
// pane call into Inspect.
package rename

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/furkandedizkan/handy-tools/internal/tools"
)

// Collision selects what Run does when a target path already exists or would
// be produced twice within the same batch.
type Collision int

const (
	CollisionError  Collision = iota // default — abort with BAD_REQUEST
	CollisionSkip                    // skip the entry, continue
	CollisionSuffix                  // append -1, -2, ... like image.disambiguatePath
)

// Request describes a rename batch. Sources are absolute or working-dir
// relative file paths — the caller is responsible for listing the directory
// and passing in the candidates. The package never walks directories itself,
// which keeps the path-safety surface on the caller (HTTP layer checks each
// path against allow-roots).
type Request struct {
	Sources     []string
	Pattern     string
	Replace     string
	OnCollision Collision
}

// Plan is one row of a rename batch — the source path and the path it would
// be moved to. From == To means the basename didn't match the pattern (or
// the replacement is a no-op); those rows aren't actioned by Run.
type Plan struct {
	From string
	To   string
}

// Inspect compiles the pattern, walks the source list, and returns the
// rename plan plus any collisions detected within the batch. It performs no
// filesystem mutation. Used as the preflight for --dry-run and the UI
// preview pane.
func Inspect(req Request) ([]Plan, *tools.Error) {
	if req.Pattern == "" {
		return nil, &tools.Error{Code: tools.CodeBadRequest, Message: "pattern is required"}
	}
	re, err := regexp.Compile(req.Pattern)
	if err != nil {
		return nil, &tools.Error{
			Code:    tools.CodeBadRequest,
			Message: "invalid regex pattern",
			Detail:  err.Error(),
		}
	}
	plans := make([]Plan, 0, len(req.Sources))
	for _, src := range req.Sources {
		dir, base := filepath.Split(src)
		if !re.MatchString(base) {
			plans = append(plans, Plan{From: src, To: src})
			continue
		}
		newBase := re.ReplaceAllString(base, req.Replace)
		if newBase == base {
			plans = append(plans, Plan{From: src, To: src})
			continue
		}
		plans = append(plans, Plan{From: src, To: filepath.Join(dir, newBase)})
	}
	return plans, nil
}

// Run executes the rename batch and streams progress on the returned channel.
// The channel is closed when the batch completes (success or failure). The
// terminal event carries the renamed / skipped / failed counts in its
// Message and an *tools.Error when nothing succeeded.
func Run(ctx context.Context, req Request) <-chan tools.Progress {
	ch := make(chan tools.Progress, 8)
	go func() {
		defer close(ch)
		started := time.Now()
		emit := func(p tools.Progress) {
			p.Tool = "rename"
			p.Action = "run"
			p.StartedAt = started
			select {
			case <-ctx.Done():
			case ch <- p:
			}
		}

		if len(req.Sources) == 0 {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeBadRequest, Message: "rename needs at least one source",
			}})
			return
		}

		plans, terr := Inspect(req)
		if terr != nil {
			emit(tools.Progress{Completed: true, Err: terr})
			return
		}

		// Pre-pass: resolve in-batch collisions and existing-target collisions
		// up front so we don't half-move a batch only to abort. Mutates plans
		// in place when OnCollision == CollisionSuffix.
		if err := resolveCollisions(plans, req.OnCollision); err != nil {
			emit(tools.Progress{Completed: true, Err: err})
			return
		}

		var renamed, skipped, failed int
		total := len(plans)
		for i, p := range plans {
			if err := ctx.Err(); err != nil {
				emit(tools.Progress{Completed: true, Err: &tools.Error{
					Code: tools.CodeAborted, Message: "rename canceled",
				}})
				return
			}
			if p.From == p.To {
				skipped++
				emit(tools.Progress{
					Level:       tools.SeverityInfo,
					CurrentItem: filepath.Base(p.From),
					Fraction:    float64(i+1) / float64(total),
					Message:     fmt.Sprintf("[%d/%d] skip %s (no match)", i+1, total, filepath.Base(p.From)),
				})
				continue
			}
			if err := os.Rename(p.From, p.To); err != nil {
				failed++
				code := tools.ClassifyFSError(err)
				emit(tools.Progress{
					Level:       tools.SeverityError,
					CurrentItem: filepath.Base(p.From),
					Fraction:    float64(i+1) / float64(total),
					Message:     fmt.Sprintf("[%d/%d] failed %s -> %s: %s (%s)", i+1, total, filepath.Base(p.From), filepath.Base(p.To), err.Error(), code),
					Err:         &tools.Error{Code: code, Message: "rename failed", Detail: err.Error()},
				})
				continue
			}
			renamed++
			emit(tools.Progress{
				Level:       tools.SeverityInfo,
				CurrentItem: filepath.Base(p.To),
				Fraction:    float64(i+1) / float64(total),
				Message:     fmt.Sprintf("[%d/%d] %s -> %s", i+1, total, filepath.Base(p.From), filepath.Base(p.To)),
			})
		}

		summary := fmt.Sprintf("renamed %d, skipped %d, failed %d", renamed, skipped, failed)
		if failed == total {
			emit(tools.Progress{Completed: true, Err: &tools.Error{
				Code: tools.CodeIO, Message: "all renames failed", Detail: summary,
			}})
			return
		}
		emit(tools.Progress{Completed: true, Fraction: 1, Level: tools.SeverityInfo, Message: summary})
	}()
	return ch
}

// resolveCollisions inspects the plan and either errors / skips / suffixes
// when a target already exists on disk or another plan row would land on
// the same destination. Modifies p.To in place for CollisionSuffix entries.
// The `taken` map carries in-batch reservations forward so suffix
// disambiguation accounts for siblings that haven't been written yet.
func resolveCollisions(plans []Plan, mode Collision) *tools.Error {
	taken := make(map[string]int, len(plans))
	for i, p := range plans {
		if p.From == p.To {
			continue
		}
		// Either in-batch reservation or existing on disk is a collision.
		_, prev, batchHit := lookupBatch(taken, p.To)
		diskHit, statErr := existsOnDisk(p.To)
		if statErr != nil {
			return &tools.Error{Code: tools.ClassifyFSError(statErr), Message: "stat target", Detail: statErr.Error()}
		}
		if !batchHit && !diskHit {
			taken[p.To] = i
			continue
		}
		switch mode {
		case CollisionSkip:
			plans[i].To = plans[i].From
		case CollisionSuffix:
			newTo, err := disambiguateAgainst(p.To, taken)
			if err != nil {
				return err
			}
			plans[i].To = newTo
			taken[newTo] = i
		default:
			detail := p.To
			if batchHit {
				detail = fmt.Sprintf("%s and %s -> %s", plans[prev].From, plans[i].From, p.To)
			}
			msg := "target exists"
			if batchHit {
				msg = "two sources would rename to the same target"
			}
			return &tools.Error{Code: tools.CodeBadRequest, Message: msg, Detail: detail}
		}
	}
	return nil
}

func lookupBatch(taken map[string]int, path string) (string, int, bool) {
	if prev, ok := taken[path]; ok {
		return path, prev, true
	}
	return "", 0, false
}

func existsOnDisk(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

// disambiguateAgainst appends -1, -2, ... before the extension until a free
// name is found, treating both the filesystem and the in-batch `taken` map
// as occupied. Capped at 9999 so a pathological directory can't spin forever.
func disambiguateAgainst(path string, taken map[string]int) (string, *tools.Error) {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	const max = 9999
	for i := 1; i <= max; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, hit := taken[candidate]; hit {
			continue
		}
		exists, err := existsOnDisk(candidate)
		if err != nil {
			return "", &tools.Error{Code: tools.ClassifyFSError(err), Message: "stat candidate", Detail: err.Error()}
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", &tools.Error{
		Code:    tools.CodeBadRequest,
		Message: "could not find a free suffix",
		Detail:  fmt.Sprintf("tried %s-1%s .. %s-%d%s", base, ext, base, max, ext),
	}
}
