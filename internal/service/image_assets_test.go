package service

import "testing"

func TestParseReplyChunks(t *testing.T) {
	chunks := ParseReplyChunks("给你这张[[image:good_night_cat]]晚安")
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].Text != "给你这张" {
		t.Fatalf("unexpected text chunk: %#v", chunks[0])
	}
	if chunks[1].ImageAssetID != "good_night_cat" {
		t.Fatalf("unexpected image asset chunk: %#v", chunks[1])
	}
	if chunks[2].Text != "晚安" {
		t.Fatalf("unexpected trailing text chunk: %#v", chunks[2])
	}
}

func TestStripReplyDirectives(t *testing.T) {
	result := StripReplyDirectives("给你这张[[image:good_night_cat]] 晚安")
	if result != "给你这张 晚安" {
		t.Fatalf("unexpected stripped result: %q", result)
	}
}
