package slack

import (
	"strconv"
)

// SearchResult holds search results.
type SearchResult struct {
	Messages []MessageSummary `json:"messages,omitempty"`
	Files    []FileSummary    `json:"files,omitempty"`
	Total    int              `json:"total"`
}

// SearchMessages searches messages via search.messages API.
func (c *Client) SearchMessages(query string, count int) (*SearchResult, error) {
	resp, err := c.API("search.messages", map[string]string{
		"query": query,
		"count": strconv.Itoa(count),
		"sort":  "timestamp",
	})
	if err != nil {
		return nil, err
	}

	result := &SearchResult{}
	msgs := toRecord(resp["messages"])
	matches := getArray(msgs, "matches")
	for _, m := range matches {
		rec := toRecord(m)
		channelRec := toRecord(rec["channel"])
		channelID := stringVal(channelRec, "id")
		msg := parseMessage(rec, channelID)
		msg.ChannelName = stringVal(channelRec, "name")
		result.Messages = append(result.Messages, *msg)
	}
	if total, ok := msgs["total"].(float64); ok {
		result.Total = int(total)
	}
	return result, nil
}

// SearchFiles searches files via search.files API.
func (c *Client) SearchFiles(query string, count int) (*SearchResult, error) {
	resp, err := c.API("search.files", map[string]string{
		"query": query,
		"count": strconv.Itoa(count),
	})
	if err != nil {
		return nil, err
	}

	result := &SearchResult{}
	files := toRecord(resp["files"])
	matches := getArray(files, "matches")
	for _, f := range matches {
		fm := toRecord(f)
		result.Files = append(result.Files, FileSummary{
			ID:       stringVal(fm, "id"),
			Name:     stringVal(fm, "name"),
			Filetype: stringVal(fm, "filetype"),
			Mimetype: stringVal(fm, "mimetype"),
			Size:     intVal(fm, "size"),
			URL:      stringVal(fm, "url_private"),
		})
	}
	if total, ok := files["total"].(float64); ok {
		result.Total = int(total)
	}
	return result, nil
}
