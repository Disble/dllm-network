package tray

import (
	"fmt"

	"ollama-telemetry/internal/events"
)

const TopicLifecycleCommand = "tray.lifecycle.command"

type Action string

const (
	ActionShow   Action = "show"
	ActionHide   Action = "hide"
	ActionPause  Action = "pause"
	ActionResume Action = "resume"
	ActionQuit   Action = "quit"
)

type Command struct {
	Action Action
}

type Runtime interface {
	Show() error
	Hide() error
	Pause() error
	Resume() error
	Quit() error
}

type Host struct {
	runtime Runtime
	bus     *events.Bus
}

func NewHost(runtime Runtime, bus *events.Bus) *Host {
	if bus == nil {
		bus = events.NewBus()
	}

	return &Host{runtime: runtime, bus: bus}
}

func (host *Host) Handle(action Action) error {
	if err := host.execute(action); err != nil {
		return err
	}

	host.bus.Publish(events.Event{
		Topic:   TopicLifecycleCommand,
		Payload: Command{Action: action},
	})

	return nil
}

func (host *Host) execute(action Action) error {
	switch action {
	case ActionShow:
		return host.runtime.Show()
	case ActionHide:
		return host.runtime.Hide()
	case ActionPause:
		return host.runtime.Pause()
	case ActionResume:
		return host.runtime.Resume()
	case ActionQuit:
		return host.runtime.Quit()
	default:
		return fmt.Errorf("unsupported tray action: %s", action)
	}
}
