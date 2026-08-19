package slack

import (
	"strconv"
)

// SavedItem is one entry from the user's "Later" list (saved messages).
type SavedItem struct {
	ItemType    string          `json:"item_type"`
	ItemID      string          `json:"item_id,omitempty"`    // raw id for non-message items
	ChannelID   string          `json:"channel_id,omitempty"` // set when item_type is "message"
	ChannelName string          `json:"channel_name,omitempty"`
	TS          string          `json:"ts,omitempty"`
	State       string          `json:"state,omitempty"` // in_progress, completed, archived
	IsArchived  bool            `json:"is_archived,omitempty"`
	DateCreated int             `json:"date_created,omitempty"`
	DateDue     int             `json:"date_due,omitempty"`
	Message     *MessageSummary `json:"message,omitempty"`
}

// SavedCounts summarizes the user's Later list.
type SavedCounts struct {
	Total       int `json:"total"`
	Uncompleted int `json:"uncompleted"`
	Completed   int `json:"completed"`
	Archived    int `json:"archived"`
}

// SavedList holds saved items plus summary counts.
type SavedList struct {
	Counts SavedCounts `json:"counts"`
	Items  []SavedItem `json:"items"`
}

// FetchSavedItems lists the user's saved-for-later items via the saved.list API
// (the endpoint backing Slack's "Later" view; requires browser auth).
// filter can be "saved" (in progress), "completed", "archived", or "" for all.
func (c *Client) FetchSavedItems(filter string, limit int) (*SavedList, error) {
	// saved.list rejects page sizes above 50 with invalid_arguments.
	params := map[string]string{
		"limit": strconv.Itoa(min(limit, 50)),
	}
	if filter != "" {
		params["filter"] = filter
	}

	result := &SavedList{}
	for {
		resp, err := c.API("saved.list", params)
		if err != nil {
			return nil, err
		}
		counts := toRecord(resp["counts"])
		result.Counts = SavedCounts{
			Total:       intVal(counts, "total_count"),
			Uncompleted: intVal(counts, "uncompleted_count"),
			Completed:   intVal(counts, "completed_count"),
			Archived:    intVal(counts, "archived_count"),
		}
		for _, item := range getArray(resp, "saved_items") {
			result.Items = append(result.Items, parseSavedItem(toRecord(item)))
		}
		cursor := getString(resp, "response_metadata", "next_cursor")
		if cursor == "" || len(result.Items) >= limit {
			break
		}
		params["cursor"] = cursor
	}
	if len(result.Items) > limit {
		result.Items = result.Items[:limit]
	}
	return result, nil
}

func parseSavedItem(rec map[string]any) SavedItem {
	item := SavedItem{
		ItemType:    stringVal(rec, "item_type"),
		TS:          stringVal(rec, "ts"),
		State:       stringVal(rec, "state"),
		DateCreated: intVal(rec, "date_created"),
		DateDue:     intVal(rec, "date_due"),
	}
	if archived, ok := rec["is_archived"].(bool); ok {
		item.IsArchived = archived
	}
	// For message items, item_id is the channel ID.
	if item.ItemType == "message" {
		item.ChannelID = stringVal(rec, "item_id")
	} else {
		item.ItemID = stringVal(rec, "item_id")
	}
	return item
}
