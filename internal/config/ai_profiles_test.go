package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAIProfileSetRoundTripAndActiveSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	set := AIProfileSet{Active: " default ", Profiles: map[string]AIProfile{
		" default ": {AIBaseURL: " https://example.test/v1 ", AIModel: " model "},
	}}
	if err := SaveAIProfileSet(path, set); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadAIProfileSet(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	name, profile, err := ActiveAIProfile(loaded)
	if err != nil {
		t.Fatalf("active profile: %v", err)
	}
	if name != "default" || profile.AIBaseURL != "https://example.test/v1" || profile.AIModel != "model" {
		t.Fatalf("unexpected active profile: %q %#v", name, profile)
	}
}

func TestLoadAIProfileSetRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAIProfileSet(path); err == nil {
		t.Fatal("malformed profile JSON should fail")
	}
}

func TestUpsertAIProfileRejectsPathLikeName(t *testing.T) {
	set := AIProfileSet{}
	if _, err := UpsertAIProfile(&set, "../secret", AIProfile{}); err == nil {
		t.Fatal("path-like profile name should fail")
	}
}

func TestReloadRuntimeConfigRollsBackWhenAIProfileLoadFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := GetConfig()
	previousHost := cfg.Hostadd
	previousProfile := cfg.AiProfile
	t.Setenv("AI_CONFIG_FILE", path)
	t.Setenv("HOSTADD", "changed-during-failed-reload")

	if err := ReloadRuntimeConfig(); err == nil {
		t.Fatal("ReloadRuntimeConfig should fail for malformed AI profile")
	}
	if cfg.Hostadd != previousHost || cfg.AiProfile != previousProfile {
		t.Fatalf("config changed after failed reload: host=%q profile=%q", cfg.Hostadd, cfg.AiProfile)
	}
}
