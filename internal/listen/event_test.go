package listen

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const self = "U0SELF"

func mustParse(t *testing.T, s string) RawEvent {
	t.Helper()
	ev, err := ParseEvent([]byte(s))
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return ev
}

func baseFilter() Filter {
	return Filter{Types: ParseTypes("message"), IncludeBots: true, SelfID: self}
}

func TestParseEventStringAndObjectFields(t *testing.T) {
	ev := mustParse(t, `{"type":"channel_created","channel":{"id":"C1","name":"x"},"user":{"id":"U2"}}`)
	if ev.ChannelID() != "C1" || ev.UserID() != "U2" {
		t.Fatalf("got channel=%q user=%q", ev.ChannelID(), ev.UserID())
	}
	ev = mustParse(t, `{"type":"message","channel":"D1","user":"U3","text":"hi","ts":"1.2"}`)
	if ev.ChannelID() != "D1" || ev.UserID() != "U3" || ev.Text != "hi" {
		t.Fatalf("unexpected %+v", ev)
	}
}

func TestParseEventPongReplyTo(t *testing.T) {
	ev := mustParse(t, `{"type":"pong","reply_to":7}`)
	if ev.ReplyTo == nil || *ev.ReplyTo != 7 {
		t.Fatalf("reply_to not parsed: %+v", ev.ReplyTo)
	}
}

func TestApplyProjectsFields(t *testing.T) {
	ev := mustParse(t, `{"type":"message","channel":"C1","user":"U9","text":"hey <@U0SELF> look","ts":"10.1","thread_ts":"9.0","event_ts":"10.1"}`)
	rec, ok := baseFilter().Apply(ev)
	if !ok {
		t.Fatal("expected emit")
	}
	want := Record{Type: "message", Channel: "C1", User: "U9", Text: "hey <@U0SELF> look", TS: "10.1", ThreadTS: "9.0", EventTS: "10.1", IsMention: true}
	if rec != want {
		t.Fatalf("got %+v\nwant %+v", rec, want)
	}
}

func TestApplyTypesFilter(t *testing.T) {
	ev := mustParse(t, `{"type":"user_typing","channel":"D1","user":"U9"}`)
	if _, ok := baseFilter().Apply(ev); ok {
		t.Fatal("user_typing should be filtered by --types message")
	}
	f := baseFilter()
	f.Types = ParseTypes("")
	if _, ok := f.Apply(ev); !ok {
		t.Fatal("empty types should allow all")
	}
}

func TestApplyDropsControlTypes(t *testing.T) {
	f := baseFilter()
	f.Types = nil
	for _, typ := range []string{"hello", "pong", "goodbye", "reconnect_url", "error"} {
		if _, ok := f.Apply(RawEvent{Type: typ}); ok {
			t.Fatalf("%s should not be emitted", typ)
		}
	}
}

func TestApplySubtypes(t *testing.T) {
	changed := mustParse(t, `{"type":"message","subtype":"message_changed","channel":"D1","message":{"user":"U9","text":"edited","ts":"5.0"},"ts":"6.0"}`)
	deleted := mustParse(t, `{"type":"message","subtype":"message_deleted","channel":"D1","ts":"6.0"}`)
	broadcast := mustParse(t, `{"type":"message","subtype":"thread_broadcast","channel":"D1","user":"U9","text":"b"}`)

	f := baseFilter()
	if _, ok := f.Apply(changed); ok {
		t.Fatal("message_changed should be ignored by default")
	}
	if _, ok := f.Apply(deleted); ok {
		t.Fatal("message_deleted should be ignored by default")
	}
	if _, ok := f.Apply(broadcast); !ok {
		t.Fatal("thread_broadcast should be emitted")
	}

	f.AllSubtypes = true
	rec, ok := f.Apply(changed)
	if !ok || rec.Text != "edited" || rec.User != "U9" || rec.TS != "5.0" {
		t.Fatalf("message_changed should project nested message: ok=%v %+v", ok, rec)
	}
}

func TestApplyBots(t *testing.T) {
	ev := mustParse(t, `{"type":"message","subtype":"bot_message","channel":"D1","bot_id":"B1","text":"beep"}`)
	f := baseFilter()
	rec, ok := f.Apply(ev)
	if !ok || rec.BotID != "B1" {
		t.Fatalf("bots included by default: ok=%v %+v", ok, rec)
	}
	f.IncludeBots = false
	if _, ok := f.Apply(ev); ok {
		t.Fatal("bot should be dropped with include-bots=false")
	}
	plain := mustParse(t, `{"type":"message","channel":"D1","bot_id":"B1","user":"U5","text":"app msg"}`)
	if _, ok := f.Apply(plain); ok {
		t.Fatal("message with bot_id should be dropped with include-bots=false")
	}
}

func TestMentionsOrDMs(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		isMPIM   func(string) bool
		wantEmit bool
		wantDM   bool
		wantMent bool
		wantSelf bool
	}{
		{name: "dm", raw: `{"type":"message","channel":"D1","user":"U9","text":"hi"}`, wantEmit: true, wantDM: true},
		{name: "channel mention", raw: `{"type":"message","channel":"C1","user":"U9","text":"<@U0SELF> ping"}`, wantEmit: true, wantMent: true},
		{name: "mention with label", raw: `{"type":"message","channel":"C1","user":"U9","text":"cc <@U0SELF|me>"}`, wantEmit: true, wantMent: true},
		{name: "plain channel", raw: `{"type":"message","channel":"C1","user":"U9","text":"nothing"}`, wantEmit: false},
		{name: "other mention", raw: `{"type":"message","channel":"C1","user":"U9","text":"<@U0OTHER>"}`, wantEmit: false},
		{name: "G prefix alone is not a group DM (legacy private channel)", raw: `{"type":"message","channel":"G1","user":"U9","text":"group"}`, wantEmit: false},
		{name: "G prefix resolved as mpim", raw: `{"type":"message","channel":"G1","user":"U9","text":"group"}`, isMPIM: func(c string) bool { return c == "G1" }, wantEmit: true},
		{name: "longer id sharing self prefix is not a mention", raw: `{"type":"message","channel":"C1","user":"U9","text":"<@U0SELF2> hi"}`, wantEmit: false},
		{name: "explicit not mpim", raw: `{"type":"message","channel":"G1","is_mpim":false,"user":"U9","text":"private chan"}`, wantEmit: false},
		{name: "channel_type mpim", raw: `{"type":"message","channel":"C2","channel_type":"mpim","user":"U9","text":"x"}`, wantEmit: true},
		{name: "channel_type im", raw: `{"type":"message","channel":"C3","channel_type":"im","user":"U9","text":"x"}`, wantEmit: true, wantDM: true},
		{name: "resolver says mpim", raw: `{"type":"message","channel":"C7","user":"U9","text":"x"}`, isMPIM: func(c string) bool { return c == "C7" }, wantEmit: true},
		{name: "self dm", raw: `{"type":"message","channel":"D1","user":"U0SELF","text":"note"}`, wantEmit: true, wantDM: true, wantSelf: true},
		{name: "self mentions self", raw: `{"type":"message","channel":"C1","user":"U0SELF","text":"<@U0SELF> todo"}`, wantEmit: true, wantMent: true, wantSelf: true},
		{name: "self plain channel", raw: `{"type":"message","channel":"C1","user":"U0SELF","text":"hello all"}`, wantEmit: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := baseFilter()
			f.MentionsOrDMs = true
			f.IsMPIM = tc.isMPIM
			rec, ok := f.Apply(mustParse(t, tc.raw))
			if ok != tc.wantEmit {
				t.Fatalf("emit=%v want %v", ok, tc.wantEmit)
			}
			if !ok {
				return
			}
			if rec.IsDM != tc.wantDM || rec.IsMention != tc.wantMent || rec.FromSelf != tc.wantSelf {
				t.Fatalf("flags dm=%v mention=%v self=%v", rec.IsDM, rec.IsMention, rec.FromSelf)
			}
		})
	}
}

func TestTextTruncatedTo2000Runes(t *testing.T) {
	long := strings.Repeat("中", MaxTextRunes+50)
	ev := RawEvent{Type: "message", Channel: []byte(`"D1"`), Text: long}
	rec, ok := baseFilter().Apply(ev)
	if !ok {
		t.Fatal("expected emit")
	}
	if n := len([]rune(rec.Text)); n != MaxTextRunes {
		t.Fatalf("text runes=%d want %d", n, MaxTextRunes)
	}
}

func TestParseTypes(t *testing.T) {
	got := ParseTypes(" message, reaction_added ,,")
	if len(got) != 2 || !got["message"] || !got["reaction_added"] {
		t.Fatalf("got %v", got)
	}
}

func TestApplyKeepsAttachmentsAndBlocks(t *testing.T) {
	ev := mustParse(t, `{"type":"message","channel":"D1","bot_id":"B1","text":"1 minute until","ts":"10.1",`+
		`"attachments":[{"actions":[{"text":"Join","url":"https://meet.google.com/abc-defg-hij"}]}],"blocks":[{"type":"section"}]}`)
	rec, ok := baseFilter().Apply(ev)
	if !ok {
		t.Fatal("expected emit")
	}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"attachments":[{"actions":[{"text":"Join","url":"https://meet.google.com/abc-defg-hij"}]}]`, `"blocks":[{"type":"section"}]`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("missing %s in %s", want, out)
		}
	}
}

func TestSelectPassesNonMessageEventsThroughUnchanged(t *testing.T) {
	// Arrange
	raw := `{"type":"reaction_added","user":"U9","reaction":"eyes","item_user":"U0SELF",` +
		`"item":{"type":"message","channel":"C1","ts":"1700000000.000100"},"event_ts":"1700000001.000000"}`
	f := baseFilter()
	f.Types = ParseTypes("message,reaction_added")

	// Act
	line, ok := f.Select(mustParse(t, raw))

	// Assert
	if !ok {
		t.Fatal("reaction_added in --types should be emitted")
	}
	got, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	var want, have map[string]any
	_ = json.Unmarshal([]byte(raw), &want)
	_ = json.Unmarshal(got, &have)
	if !reflect.DeepEqual(want, have) {
		t.Fatalf("non-message event changed:\n got %s\nwant %s", got, raw)
	}
}

func TestSelectProjectsMessageEvents(t *testing.T) {
	line, ok := baseFilter().Select(mustParse(t, `{"type":"message","channel":"D1","user":"U9","text":"hi","extra":"dropped"}`))
	if !ok {
		t.Fatal("message should be emitted")
	}
	rec, isRecord := line.(Record)
	if !isRecord || rec.Channel != "D1" || !rec.IsDM {
		t.Fatalf("want projected Record, got %#v", line)
	}
}

func TestSelectStillAppliesTypesFilterToNonMessageEvents(t *testing.T) {
	if _, ok := baseFilter().Select(mustParse(t, `{"type":"reaction_added","user":"U9"}`)); ok {
		t.Fatal("reaction_added not in --types should be dropped")
	}
}
