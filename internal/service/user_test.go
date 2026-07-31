package service

import (
	"testing"

	"project-yume/internal/config"
)

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
