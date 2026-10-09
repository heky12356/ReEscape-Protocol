package app

import (
	"testing"
	"time"
)

func TestDurationMsUsesFallbackForNonPositiveValues(t *testing.T) {
	fallback := 250 * time.Millisecond
	if got := durationMs(0, fallback); got != fallback {
		t.Fatalf("zero duration = %s, want %s", got, fallback)
	}
	if got := durationMs(-1, fallback); got != fallback {
		t.Fatalf("negative duration = %s, want %s", got, fallback)
	}
	if got := durationMs(125, fallback); got != 125*time.Millisecond {
		t.Fatalf("configured duration = %s", got)
	}
}
