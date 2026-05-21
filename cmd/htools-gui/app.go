//go:build wails

package main

import "context"

// App is bound into the Wails frontend so Go methods are callable from
// JavaScript. #79 ships only the lifecycle hook; #80 adds the native
// file-picker and system-menu methods.
type App struct {
	ctx context.Context
}

func newApp() *App { return &App{} }

// startup captures the Wails runtime context, which later binding calls
// (dialogs, menu, window) need.
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
