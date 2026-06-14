package tray

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"ollama-telemetry/internal/events"
)

func TestHostHandlesLifecycleActionsAndPublishesCommands(t *testing.T) {
	t.Parallel()

	bus := events.NewBus()
	runtime := &fakeRuntime{}
	host := NewHost(runtime, bus)

	var (
		mu       sync.Mutex
		commands []Command
	)
	unsubscribe := bus.Subscribe(TopicLifecycleCommand, func(event events.Event) {
		command, ok := event.Payload.(Command)
		if !ok {
			t.Fatalf("expected tray command payload, got %T", event.Payload)
		}

		mu.Lock()
		defer mu.Unlock()
		commands = append(commands, command)
	})
	defer unsubscribe()

	testCases := []struct {
		name   string
		action Action
		call   string
	}{
		{name: "show routes to runtime", action: ActionShow, call: "show"},
		{name: "hide routes to runtime", action: ActionHide, call: "hide"},
		{name: "pause routes to runtime", action: ActionPause, call: "pause"},
		{name: "resume routes to runtime", action: ActionResume, call: "resume"},
		{name: "quit routes to runtime", action: ActionQuit, call: "quit"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if err := host.Handle(tt.action); err != nil {
				t.Fatalf("handle %s: %v", tt.action, err)
			}

			if got := runtime.calls[len(runtime.calls)-1]; got != tt.call {
				t.Fatalf("expected runtime call %q, got %q", tt.call, got)
			}

			mu.Lock()
			defer mu.Unlock()
			if got := commands[len(commands)-1].Action; got != tt.action {
				t.Fatalf("expected published action %q, got %q", tt.action, got)
			}
		})
	}

	if !slices.Equal(runtime.calls, []string{"show", "hide", "pause", "resume", "quit"}) {
		t.Fatalf("expected runtime calls for all lifecycle actions, got %v", runtime.calls)
	}
}

func TestHostRejectsUnsupportedOrFailedActions(t *testing.T) {
	t.Parallel()

	t.Run("unsupported action returns error without publish", func(t *testing.T) {
		bus := events.NewBus()
		runtime := &fakeRuntime{}
		host := NewHost(runtime, bus)

		published := false
		unsubscribe := bus.Subscribe(TopicLifecycleCommand, func(events.Event) {
			published = true
		})
		defer unsubscribe()

		err := host.Handle(Action("invalid"))
		if err == nil {
			t.Fatal("expected invalid tray action to fail")
		}

		if published {
			t.Fatal("expected invalid tray action to avoid publishing")
		}
	})

	t.Run("runtime failure prevents publish", func(t *testing.T) {
		bus := events.NewBus()
		runtime := &fakeRuntime{failures: map[Action]error{ActionQuit: errors.New("quit failed")}}
		host := NewHost(runtime, bus)

		published := false
		unsubscribe := bus.Subscribe(TopicLifecycleCommand, func(events.Event) {
			published = true
		})
		defer unsubscribe()

		err := host.Handle(ActionQuit)
		if err == nil || err.Error() != "quit failed" {
			t.Fatalf("expected quit failure, got %v", err)
		}

		if published {
			t.Fatal("expected failed runtime action to avoid publishing")
		}
	})
}

type fakeRuntime struct {
	calls    []string
	failures map[Action]error
}

func (runtime *fakeRuntime) Show() error {
	return runtime.call(ActionShow)
}

func (runtime *fakeRuntime) Hide() error {
	return runtime.call(ActionHide)
}

func (runtime *fakeRuntime) Pause() error {
	return runtime.call(ActionPause)
}

func (runtime *fakeRuntime) Resume() error {
	return runtime.call(ActionResume)
}

func (runtime *fakeRuntime) Quit() error {
	return runtime.call(ActionQuit)
}

func (runtime *fakeRuntime) call(action Action) error {
	runtime.calls = append(runtime.calls, string(action))
	if runtime.failures == nil {
		return nil
	}
	return runtime.failures[action]
}
