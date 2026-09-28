package service

import (
	"encoding/json"
	"testing"

	"project-yume/internal/config"
)

func TestParseOutboundMessagesExpandsTextImagesAndSegments(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = false
	defer func() { cfg.EnableSpaceSegmentDelimiter = previous }()

	items := ParseOutboundMessages("给你这张[[image:hamster_glasses]]晚安$早点睡")
	if len(items) != 4 {
		t.Fatalf("expected 4 outbound items, got %d", len(items))
	}
	if items[0].Kind != "text" || items[0].Content != "给你这张" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Kind != "image" || items[1].AssetID != "hamster_glasses" {
		t.Fatalf("unexpected image item: %#v", items[1])
	}
	if items[2].Content != "晚安" || items[3].Content != "早点睡" {
		t.Fatalf("unexpected text items: %#v", items)
	}
}

func TestParseMessageIDAcceptsNumberAndString(t *testing.T) {
	for _, raw := range []string{`{"message_id":123}`, `{"message_id":"456"}`} {
		if got := parseMessageID(json.RawMessage(raw)); got == 0 {
			t.Fatalf("expected message id from %s", raw)
		}
	}
}

func TestSplitReplySegmentsDollarByDefault(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = false
	defer func() {
		cfg.EnableSpaceSegmentDelimiter = previous
	}()

	segments := splitReplySegments("第一句$第二句$第三句")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
}

func TestSplitReplySegmentsBlankLineByDefault(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = false
	defer func() { cfg.EnableSpaceSegmentDelimiter = previous }()

	segments := splitReplySegments("第一段\n\n第二段\r\n\r\n第三段")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %#v", len(segments), segments)
	}
	if segments[0] != "第一段" || segments[1] != "第二段" || segments[2] != "第三段" {
		t.Fatalf("unexpected segments: %#v", segments)
	}
}

func TestSplitReplySegmentsKeepsSingleNewline(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = false
	defer func() { cfg.EnableSpaceSegmentDelimiter = previous }()

	segments := splitReplySegments("第一行\n第二行")
	if len(segments) != 1 || segments[0] != "第一行\n第二行" {
		t.Fatalf("expected single newline to remain in one segment, got %#v", segments)
	}
}

func TestSplitReplySegmentsSupportsDollarAndBlankLineTogether(t *testing.T) {
	segments := splitReplySegments("第一段$第二段\n\n第三段")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %#v", len(segments), segments)
	}
}

func TestSplitReplySegmentsDoesNotUseSpaceByDefault(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = false
	defer func() {
		cfg.EnableSpaceSegmentDelimiter = previous
	}()

	segments := splitReplySegments("第一句 第二句 第三句")
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment when space delimiter disabled, got %d", len(segments))
	}
}

func TestSplitReplySegmentsUsesSpaceWhenEnabled(t *testing.T) {
	cfg := config.GetConfig()
	previous := cfg.EnableSpaceSegmentDelimiter
	cfg.EnableSpaceSegmentDelimiter = true
	defer func() {
		cfg.EnableSpaceSegmentDelimiter = previous
	}()

	segments := splitReplySegments("第一句 第二句 第三句")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments when space delimiter enabled, got %d", len(segments))
	}
}
