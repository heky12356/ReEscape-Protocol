package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func setupRuntimeReloadTest(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ai_profiles.json")
	set := AIProfileSet{
		Active: "default",
		Profiles: map[string]AIProfile{
			"default": {
				AIBaseURL:     "https://example.test/v1",
				AIModel:       "test-model",
				AIMaxTokens:   2000,
				AITimeout:     30,
				AIRetryCount:  0,
				AIRateLimit:   20,
				AITemperature: 1,
				AITopP:        0.9,
			},
		},
	}
	data, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_CONFIG_FILE", path)
}

func TestValidateConfigRejectsInvalidRuntimeLimits(t *testing.T) {
	cfg := cloneConfig(GetConfig())
	cfg.ReactMaxSteps = 0
	if err := validateConfig(cfg); err == nil {
		t.Fatal("expected invalid react limits to be rejected")
	}
}

func TestReloadRuntimeConfigCommitsVersionAndScope(t *testing.T) {
	setupRuntimeReloadTest(t)
	clearReloadCallbacks()
	t.Cleanup(clearReloadCallbacks)
	callbackCalled := false
	RegisterReloadCallback(ScopeBot, func(change ReloadChange) error {
		callbackCalled = true
		if GetConfig().Version != change.Version {
			t.Fatalf("callback observed version %d, want %d", GetConfig().Version, change.Version)
		}
		return nil
	})
	previousEnv := os.Getenv("HOSTADD")
	t.Setenv("HOSTADD", "epoch1-test-host")
	t.Cleanup(func() {
		_ = os.Setenv("HOSTADD", previousEnv)
		_ = ReloadRuntimeConfig()
	})

	before := GetConfig()
	previousVersion := before.Version
	if err := ReloadRuntimeConfig(); err != nil {
		t.Fatalf("reload runtime config: %v", err)
	}
	after := GetConfig()
	if after.Version != previousVersion+1 {
		t.Fatalf("version = %d, want %d", after.Version, previousVersion+1)
	}
	if after.Hostadd != "epoch1-test-host" || !containsScope(after.LastReloadScopes, ScopeBot) {
		t.Fatalf("unexpected committed config: host=%q scopes=%v", after.Hostadd, after.LastReloadScopes)
	}
	if !callbackCalled {
		t.Fatal("expected reload callback")
	}
}

func TestReloadRuntimeConfigCallbackFailureKeepsPreviousConfig(t *testing.T) {
	setupRuntimeReloadTest(t)
	clearReloadCallbacks()
	t.Cleanup(clearReloadCallbacks)
	RegisterReloadCallback(ScopeBot, func(ReloadChange) error {
		return errors.New("callback failed")
	})

	previousEnv := os.Getenv("HOSTADD")
	t.Setenv("HOSTADD", "epoch1-callback-failure")
	t.Cleanup(func() {
		_ = os.Setenv("HOSTADD", previousEnv)
		_ = ReloadRuntimeConfig()
	})

	before := GetConfig()
	if err := ReloadRuntimeConfig(); err == nil {
		t.Fatal("expected callback failure")
	}
	if after := GetConfig(); after != before || after.Hostadd != before.Hostadd || after.Version != before.Version {
		t.Fatalf("configuration changed after callback failure: before=%p/%+v after=%p/%+v", before, before, after, after)
	}
}

func TestReloadRuntimeConfigSkillFailureKeepsPreviousState(t *testing.T) {
	setupRuntimeReloadTest(t)
	previousEnv := os.Getenv("SKILL_DIRS")
	t.Setenv("SKILL_DIRS", filepath.Join(t.TempDir(), "missing-skills"))
	t.Cleanup(func() {
		_ = os.Setenv("SKILL_DIRS", previousEnv)
		_ = ReloadRuntimeConfig()
	})

	before := GetConfig()
	if err := ReloadRuntimeConfig(); err == nil {
		t.Fatal("expected missing skill directory to fail reload")
	}
	if after := GetConfig(); after != before || after.Version != before.Version {
		t.Fatalf("configuration changed after skill reload failure: before=%p after=%p", before, after)
	}
}
