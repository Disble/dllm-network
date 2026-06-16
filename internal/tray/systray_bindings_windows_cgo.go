//go:build windows && cgo

package tray

import "github.com/getlantern/systray"

func init() {
	runWithExternalLoop = func(onReady, onExit func()) {
		go systray.Run(onReady, onExit)
	}
	setIcon = systray.SetIcon
	setTooltip = systray.SetTooltip
	addMenuItem = func(title, tooltip string) menuItem {
		return systrayMenuItem{item: systray.AddMenuItem(title, tooltip)}
	}
	quit = systray.Quit
}

type systrayMenuItem struct {
	item *systray.MenuItem
}

func (i systrayMenuItem) Clicked() <-chan struct{} {
	if i.item == nil {
		return nil
	}

	return i.item.ClickedCh
}
