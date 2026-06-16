package tray

import (
	"sync"
	"testing"
	"time"
)

func TestSystrayManagerStartConfiguresMenuAndInvokesCallbacks(t *testing.T) {
	reset := stubSystrayPrimitives()
	defer reset()

	var titles []string
	var icon []byte
	var tooltip string
	items := map[string]*fakeMenuItem{}

	addMenuItem = func(title, _ string) menuItem {
		item := newFakeMenuItem()
		titles = append(titles, title)
		items[title] = item
		return item
	}
	setIcon = func(value []byte) { icon = append([]byte(nil), value...) }
	setTooltip = func(value string) { tooltip = value }
	runWithExternalLoop = func(onReady, _ func()) { onReady() }

	manager := NewSystrayManager()

	opened := make(chan struct{}, 1)
	exited := make(chan struct{}, 1)
	err := manager.Start(Config{
		Icon:    []byte{0x01, 0x02, 0x03},
		Tooltip: "Ollama Telemetry",
		OnOpen:  func() { opened <- struct{}{} },
		OnExit:  func() { exited <- struct{}{} },
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if got, want := len(titles), 2; got != want {
		t.Fatalf("expected %d menu items, got %d", want, got)
	}

	if titles[0] != "Abrir" || titles[1] != "Salir" {
		t.Fatalf("expected menu titles [Abrir Salir], got %v", titles)
	}

	if string(icon) != string([]byte{0x01, 0x02, 0x03}) {
		t.Fatalf("expected icon bytes to be forwarded, got %v", icon)
	}

	if tooltip != "Ollama Telemetry" {
		t.Fatalf("expected tooltip %q, got %q", "Ollama Telemetry", tooltip)
	}

	items["Abrir"].click()
	assertSignal(t, opened, "open callback")

	items["Salir"].click()
	assertSignal(t, exited, "exit callback")
}

func TestSystrayManagerStopIsIdempotent(t *testing.T) {
	reset := stubSystrayPrimitives()
	defer reset()

	quitCalls := 0
	quit = func() { quitCalls++ }
	runWithExternalLoop = func(onReady, _ func()) { onReady() }
	addMenuItem = func(string, string) menuItem { return newFakeMenuItem() }

	manager := NewSystrayManager()
	if err := manager.Start(Config{Icon: []byte{0x01}}); err != nil {
		t.Fatalf("expected nil start error, got %v", err)
	}

	if err := manager.Stop(); err != nil {
		t.Fatalf("expected nil first stop error, got %v", err)
	}

	if err := manager.Stop(); err != nil {
		t.Fatalf("expected nil second stop error, got %v", err)
	}

	if quitCalls != 1 {
		t.Fatalf("expected quit once, got %d", quitCalls)
	}
}

func stubSystrayPrimitives() func() {
	originalRun := runWithExternalLoop
	originalSetIcon := setIcon
	originalSetTooltip := setTooltip
	originalAddMenuItem := addMenuItem
	originalQuit := quit

	runWithExternalLoop = func(onReady, _ func()) { onReady() }
	setIcon = func([]byte) {}
	setTooltip = func(string) {}
	addMenuItem = func(string, string) menuItem { return nil }
	quit = func() {}

	return func() {
		runWithExternalLoop = originalRun
		setIcon = originalSetIcon
		setTooltip = originalSetTooltip
		addMenuItem = originalAddMenuItem
		quit = originalQuit
	}
}

type fakeMenuItem struct {
	clicked chan struct{}
	once    sync.Once
}

func newFakeMenuItem() *fakeMenuItem {
	return &fakeMenuItem{clicked: make(chan struct{}, 1)}
}

func (i *fakeMenuItem) Clicked() <-chan struct{} {
	return i.clicked
}

func (i *fakeMenuItem) click() {
	i.once.Do(func() {})
	i.clicked <- struct{}{}
}

func assertSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected %s to be triggered", label)
	}
}
