package service

import (
	"strings"
	"testing"
	"time"

	"project-yume/internal/config"
)

func TestBuildTimeContextUsesConfiguredTimezone(t *testing.T) {
	cfg := config.GetConfig()
	previousEnabled := cfg.EnableTimeContext
	previousTimezone := cfg.TimeContextTimezone
	previousFormat := cfg.TimeContextFormat
	t.Cleanup(func() {
		cfg.EnableTimeContext = previousEnabled
		cfg.TimeContextTimezone = previousTimezone
		cfg.TimeContextFormat = previousFormat
	})

	cfg.EnableTimeContext = true
	cfg.TimeContextTimezone = "Asia/Shanghai"
	cfg.TimeContextFormat = "2006-01-02 15:04"

	referenceTime := time.Date(2026, 6, 22, 13, 35, 0, 0, time.UTC)
	context := BuildTimeContext(referenceTime)

	assertContains(t, context, "【时间上下文】")
	assertContains(t, context, "当前本地时间：2026-06-22 21:35")
	assertContains(t, context, "当前时区：Asia/Shanghai")
	assertContains(t, context, "星期一")
	assertContains(t, context, "晚上")
}

func TestBuildTimeContextCanBeDisabled(t *testing.T) {
	cfg := config.GetConfig()
	previousEnabled := cfg.EnableTimeContext
	t.Cleanup(func() {
		cfg.EnableTimeContext = previousEnabled
	})

	cfg.EnableTimeContext = false

	if context := BuildTimeContext(time.Now()); context != "" {
		t.Fatalf("expected empty time context, got %q", context)
	}
}

func assertContains(t *testing.T, content, expected string) {
	t.Helper()
	if !strings.Contains(content, expected) {
		t.Fatalf("expected %q to contain %q", content, expected)
	}
}
