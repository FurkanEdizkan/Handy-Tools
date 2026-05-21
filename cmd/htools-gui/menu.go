//go:build wails

package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// appMenu builds the native window menu: a minimal File menu with Quit, plus
// the standard Edit menu so clipboard shortcuts work inside the webview.
func appMenu(a *App) *menu.Menu {
	m := menu.NewMenu()

	file := m.AddSubmenu("File")
	file.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		wailsrt.Quit(a.ctx)
	})

	m.Append(menu.EditMenu())
	return m
}
