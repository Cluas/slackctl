package slack

import "testing"

func TestParseSavedItem_Message(t *testing.T) {
	item := parseSavedItem(map[string]any{
		"item_type":    "message",
		"item_id":      "C0AAG6ZJ46T",
		"ts":           "1787104130.093579",
		"state":        "in_progress",
		"is_archived":  false,
		"date_created": float64(1787107916),
		"date_due":     float64(0),
	})
	if item.ItemType != "message" {
		t.Errorf("expected item_type message, got %q", item.ItemType)
	}
	if item.ChannelID != "C0AAG6ZJ46T" {
		t.Errorf("expected channel_id from item_id, got %q", item.ChannelID)
	}
	if item.ItemID != "" {
		t.Errorf("message items should not set item_id, got %q", item.ItemID)
	}
	if item.TS != "1787104130.093579" {
		t.Errorf("unexpected ts %q", item.TS)
	}
	if item.State != "in_progress" {
		t.Errorf("unexpected state %q", item.State)
	}
	if item.DateCreated != 1787107916 {
		t.Errorf("unexpected date_created %d", item.DateCreated)
	}
}

func TestParseSavedItem_NonMessage(t *testing.T) {
	item := parseSavedItem(map[string]any{
		"item_type":   "reminder",
		"item_id":     "Rm12345",
		"state":       "completed",
		"is_archived": true,
	})
	if item.ChannelID != "" {
		t.Errorf("non-message items should not set channel_id, got %q", item.ChannelID)
	}
	if item.ItemID != "Rm12345" {
		t.Errorf("expected item_id Rm12345, got %q", item.ItemID)
	}
	if !item.IsArchived {
		t.Error("expected is_archived true")
	}
}
