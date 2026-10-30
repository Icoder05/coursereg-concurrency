package events

import (
	"sync"
	"time"
)

// Event is what gets pushed to every connected dashboard over SSE.
type Event struct {
	ID      uint64                 `json:"id"`
	Type    string                 `json:"type"` // e.g. enrollment.created, project.assigned
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
	At      time.Time              `json:"at"`
}

// Hub fans events out to subscribers. Each subscriber gets a buffered channel;
// a slow client that fills its buffer misses events instead of blocking others
// (the UI refetches on the next event, so it self-heals).
type Hub struct {
	mu     sync.RWMutex
	subs   map[chan Event]struct{}
	closed bool
}

func NewHub() *Hub { return &Hub{subs: make(map[chan Event]struct{})} }

// Subscribe returns a channel of events and a function to unsubscribe.
// The channel is closed when the hub shuts down.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 32)
	h.mu.Lock()
	if h.closed {
		close(ch)
		h.mu.Unlock()
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			if _, ok := h.subs[ch]; ok {
				delete(h.subs, ch)
				close(ch)
			}
			h.mu.Unlock()
		})
	}
}

func (h *Hub) Publish(e Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default: // subscriber too slow; drop
		}
	}
}

// Clients reports how many dashboards are connected.
func (h *Hub) Clients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// Close disconnects all subscribers so streaming handlers return during shutdown.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for ch := range h.subs {
		close(ch)
		delete(h.subs, ch)
	}
}
