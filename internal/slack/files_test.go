package slack

import "testing"

func TestParseFileInfo(t *testing.T) {
	raw := map[string]any{
		"id":                   "F12345",
		"name":                 "photo.png",
		"title":                "My Photo",
		"filetype":             "png",
		"mimetype":             "image/png",
		"size":                 float64(12345),
		"user":                 "U12345",
		"created":              float64(1700000000),
		"url_private":          "https://files.slack.com/files-pri/T1/photo.png",
		"url_private_download": "https://files.slack.com/files-pri/T1/download/photo.png",
		"permalink":            "https://team.slack.com/files/U1/F1/photo.png",
		"channels":             []any{"C111", "C222"},
	}
	fi := parseFileInfo(raw)
	if fi.ID != "F12345" {
		t.Errorf("ID = %q, want F12345", fi.ID)
	}
	if fi.Name != "photo.png" {
		t.Errorf("Name = %q, want photo.png", fi.Name)
	}
	if fi.Title != "My Photo" {
		t.Errorf("Title = %q, want My Photo", fi.Title)
	}
	if fi.Size != 12345 {
		t.Errorf("Size = %d, want 12345", fi.Size)
	}
	if fi.Created != 1700000000 {
		t.Errorf("Created = %d, want 1700000000", fi.Created)
	}
	if fi.URLPrivateDownload != "https://files.slack.com/files-pri/T1/download/photo.png" {
		t.Errorf("URLPrivateDownload = %q", fi.URLPrivateDownload)
	}
	if len(fi.Channels) != 2 || fi.Channels[0] != "C111" {
		t.Errorf("Channels = %v, want [C111 C222]", fi.Channels)
	}
}

func TestParseFileInfo_Minimal(t *testing.T) {
	raw := map[string]any{
		"id":   "F99",
		"name": "doc.pdf",
	}
	fi := parseFileInfo(raw)
	if fi.ID != "F99" {
		t.Errorf("ID = %q, want F99", fi.ID)
	}
	if fi.Channels != nil {
		t.Errorf("Channels should be nil for empty, got %v", fi.Channels)
	}
}
