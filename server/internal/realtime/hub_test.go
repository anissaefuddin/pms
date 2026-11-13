package realtime

import "testing"

func TestHubReplayWindow(t *testing.T) {
	h := New()
	_, head, _, changed := h.After(nil)
	h.Publish(Event{ItemID: "one"})
	select {
	case <-changed:
	default:
		t.Fatal("publish did not wake waiter")
	}
	events, _, resync, _ := h.After(&head)
	if resync || len(events) != 1 || events[0].ID != 1 || events[0].ItemID != "one" {
		t.Fatalf("events=%v resync=%t", events, resync)
	}
	for range historySize {
		h.Publish(Event{ItemID: "next"})
	}
	if _, _, resync, _ := h.After(&head); !resync {
		t.Fatal("expired cursor must resync")
	}
	head++
	if events, _, resync, _ := h.After(&head); resync || len(events) != historySize {
		t.Fatalf("window len=%d resync=%t", len(events), resync)
	}
}

// A client that was away for more than the history (1000 events), or holds an id from the future, must reload instead of resuming.
func TestAfterAsksForResyncBeyondHistory(t *testing.T) {
	h := New()
	for range historySize + 1 {
		h.Publish(Event{Type: "item.updated"})
	}
	for name, cursor := range map[string]uint64{"too old": 0, "future": 5000} {
		if events, _, resync, _ := h.After(&cursor); !resync || len(events) != 0 {
			t.Errorf("%s: resync=%t events=%d", name, resync, len(events))
		}
	}
	recent := uint64(historySize)
	if events, _, resync, _ := h.After(&recent); resync || len(events) != 1 {
		t.Errorf("recent: resync=%t events=%d", resync, len(events))
	}
}
