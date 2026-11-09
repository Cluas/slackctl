// Package listen implements a real-time Slack event stream over the RTM
// websocket, emitting filtered events as JSON Lines.
package listen

import (
	"encoding/json"
	"strings"
)

// MaxTextRunes caps the text field of each emitted record.
const MaxTextRunes = 2000

// RawEvent is the subset of an RTM event payload used for filtering/output.
type RawEvent struct {
	Type     string          `json:"type"`
	Subtype  string          `json:"subtype"`
	Channel  json.RawMessage `json:"channel"` // string for messages, object for some events
	ChannelT string          `json:"channel_type"`
	TS       string          `json:"ts"`
	ThreadTS string          `json:"thread_ts"`
	User     json.RawMessage `json:"user"` // string for messages, object for user_change
	BotID    string          `json:"bot_id"`
	Text     string          `json:"text"`
	EventTS  string          `json:"event_ts"`
	IsMPIM   *bool           `json:"is_mpim"`
	ReplyTo  *int64          `json:"reply_to"`
	URL      string          `json:"url"`
	Error    *struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	} `json:"error"`
	Message *RawEvent `json:"message"` // nested for message_changed
}

// ParseEvent decodes one websocket frame into a RawEvent.
func ParseEvent(data []byte) (RawEvent, error) {
	var ev RawEvent
	err := json.Unmarshal(data, &ev)
	return ev, err
}

// ChannelID returns the channel id whether "channel" is a string or object.
func (e RawEvent) ChannelID() string {
	return idFromRaw(e.Channel)
}

// UserID returns the user id whether "user" is a string or object.
func (e RawEvent) UserID() string {
	return idFromRaw(e.User)
}

func idFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var obj struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return obj.ID
	}
	return ""
}

// Record is one emitted JSON line.
type Record struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype,omitempty"`
	Channel   string `json:"channel,omitempty"`
	TS        string `json:"ts,omitempty"`
	ThreadTS  string `json:"thread_ts,omitempty"`
	User      string `json:"user,omitempty"`
	BotID     string `json:"bot_id,omitempty"`
	Text      string `json:"text,omitempty"`
	FromSelf  bool   `json:"from_self"`
	IsMention bool   `json:"is_mention"`
	IsDM      bool   `json:"is_dm"`
	EventTS   string `json:"event_ts,omitempty"`
}

// Filter controls which events are emitted.
type Filter struct {
	Types         map[string]bool // event types to emit; empty = all
	MentionsOrDMs bool
	IncludeBots   bool
	AllSubtypes   bool
	SelfID        string
	// IsMPIM resolves whether a non-D/G channel is a group DM. May be nil.
	IsMPIM func(channel string) bool
}

// allowedMessageSubtypes are message subtypes that represent new content and
// are emitted without --all-subtypes.
var allowedMessageSubtypes = map[string]bool{
	"":                 true,
	"bot_message":      true,
	"thread_broadcast": true,
	"file_share":       true,
	"me_message":       true,
}

// ParseTypes turns a comma-separated list into a type set.
func ParseTypes(s string) map[string]bool {
	set := map[string]bool{}
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			set[t] = true
		}
	}
	return set
}

// Apply decides whether ev should be emitted and returns the projected record.
func (f Filter) Apply(ev RawEvent) (Record, bool) {
	if ev.Type == "" || isControlType(ev.Type) {
		return Record{}, false
	}
	if len(f.Types) > 0 && !f.Types[ev.Type] {
		return Record{}, false
	}
	rec := f.project(ev)
	if ev.Type != "message" {
		// Non-message events are not subject to message-level filters.
		return rec, true
	}
	if !f.AllSubtypes && !allowedMessageSubtypes[ev.Subtype] {
		return Record{}, false
	}
	if !f.IncludeBots && (rec.BotID != "" || ev.Subtype == "bot_message") {
		return Record{}, false
	}
	if f.MentionsOrDMs && !rec.IsDM && !rec.IsMention && !f.isGroupDM(ev) {
		return Record{}, false
	}
	return rec, true
}

func (f Filter) project(ev RawEvent) Record {
	src := ev
	// message_changed etc. carry the content in a nested "message".
	if ev.Message != nil {
		src = *ev.Message
	}
	channel := ev.ChannelID()
	user := src.UserID()
	rec := Record{
		Type:     ev.Type,
		Subtype:  ev.Subtype,
		Channel:  channel,
		TS:       firstNonEmpty(src.TS, ev.TS),
		ThreadTS: firstNonEmpty(src.ThreadTS, ev.ThreadTS),
		User:     user,
		BotID:    firstNonEmpty(src.BotID, ev.BotID),
		Text:     truncateRunes(src.Text, MaxTextRunes),
		EventTS:  ev.EventTS,
	}
	rec.FromSelf = f.SelfID != "" && user == f.SelfID
	rec.IsDM = strings.HasPrefix(channel, "D") || ev.ChannelT == "im"
	rec.IsMention = f.SelfID != "" && strings.Contains(src.Text, "<@"+f.SelfID)
	return rec
}

// isGroupDM: explicit is_mpim / channel_type wins; G-prefixed ids are treated
// as group DMs; otherwise ask the resolver (if any).
func (f Filter) isGroupDM(ev RawEvent) bool {
	if ev.IsMPIM != nil {
		return *ev.IsMPIM
	}
	if ev.ChannelT == "mpim" {
		return true
	}
	channel := ev.ChannelID()
	if strings.HasPrefix(channel, "G") {
		return true
	}
	if f.IsMPIM != nil && channel != "" {
		return f.IsMPIM(channel)
	}
	return false
}

func isControlType(t string) bool {
	switch t {
	case "hello", "pong", "goodbye", "reconnect_url", "error":
		return true
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
