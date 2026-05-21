//go:build wails

package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is bound into the Wails frontend so its exported methods are callable
// from JavaScript as window.go.main.App.<Method>. The web-side wrapper that
// feature-detects these is web/src/lib/native.ts (#81).
type App struct {
	ctx context.Context
}

func newApp() *App { return &App{} }

// startup captures the Wails runtime context, which every binding below needs.
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// PickFiles opens a native multi-file picker and returns the chosen absolute
// paths. The slice is empty when the user cancels.
func (a *App) PickFiles() ([]string, error) {
	return wailsrt.OpenMultipleFilesDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title: "Select files",
	})
}

// PickFolder opens a native directory picker and returns the chosen absolute
// path. The string is empty when the user cancels.
func (a *App) PickFolder() (string, error) {
	return wailsrt.OpenDirectoryDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title: "Select a folder",
	})
}

// Reveal opens the host's file manager showing path (the output of a finished
// job). On Linux the containing folder is opened; macOS/Windows select the
// file itself.
func (a *App) Reveal(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,"+path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("reveal %s: %w", path, err)
	}
	return nil
}
