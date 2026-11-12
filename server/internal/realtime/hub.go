package realtime

import "sync"

const historySize = 1000

// Event contains routing information only. The HTTP transport must authorize
// the recipient against the current Item before delivering it.
type Event struct {
	ID                     uint64
	Type, EntityID, ItemID string
	RecipientID            string
	Version                int
	Locations              []Location
	AffectedItems          []ItemVersion
}

type ItemVersion struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type Location struct{ SpaceID, ParentID string }

type Hub struct {
	mu      sync.Mutex
	next    uint64
	ring    [historySize]Event
	changed chan struct{}
}

func New() *Hub { return &Hub{changed: make(chan struct{})} }

func (h *Hub) Publish(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	event.ID = h.next
	h.ring[(h.next-1)%historySize] = event
	close(h.changed)
	h.changed = make(chan struct{})
}

// After snapshots available events and returns a notification channel for
// subsequent publications. A nil cursor starts at the current head.
func (h *Hub) After(cursor *uint64) (events []Event, head uint64, resync bool, changed <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	head, changed = h.next, h.changed
	if cursor == nil {
		return nil, head, false, changed
	}
	if *cursor > head || head-*cursor > historySize {
		return nil, head, true, changed
	}
	for id := *cursor + 1; id <= head; id++ {
		events = append(events, h.ring[(id-1)%historySize])
	}
	return events, head, false, changed
}
