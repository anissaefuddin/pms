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
