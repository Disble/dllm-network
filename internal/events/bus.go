package events

import "sync"

type Event struct {
	Topic   string
	Payload any
}

type Handler func(Event)

type Bus struct {
	mu          sync.RWMutex
	nextID      int
	subscribers map[string]map[int]Handler
}

func NewBus() *Bus {
	return &Bus{
		subscribers: map[string]map[int]Handler{},
	}
}

func (bus *Bus) Publish(event Event) {
	bus.mu.RLock()
	handlers := make([]Handler, 0, len(bus.subscribers[event.Topic]))
	for _, handler := range bus.subscribers[event.Topic] {
		handlers = append(handlers, handler)
	}
	bus.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

func (bus *Bus) Subscribe(topic string, handler Handler) func() {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	if bus.subscribers[topic] == nil {
		bus.subscribers[topic] = map[int]Handler{}
	}

	id := bus.nextID
	bus.nextID++
	bus.subscribers[topic][id] = handler

	return func() {
		bus.mu.Lock()
		defer bus.mu.Unlock()

		delete(bus.subscribers[topic], id)
		if len(bus.subscribers[topic]) == 0 {
			delete(bus.subscribers, topic)
		}
	}
}
