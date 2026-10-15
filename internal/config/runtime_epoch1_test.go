package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestValidateConfigPersistenceSettings(t *testing.T) {
	cfg := cloneConfig(GetConfig())
	// Keep this validation test independent of developer-local .env files and
	// ignored AI profile overrides that are unavailable in CI.
	cfg.AiProfile = "default"
	cfg.AiBaseUrl = "https://example.test/v1"
	cfg.AiModel = "test-model"
	cfg.AiTemperature = 1
	cfg.AiMaxTokens = 2000
	cfg.AiTimeout = 30
	cfg.AiRetryCount = 0
	cfg.AiRateLimit = 20
	cfg.AiTopP = 0.9
	cfg.PersistenceBackend = "file"
	cfg.SQLitePath = filepath.Join(t.TempDir(), "runtime.sqlite")
	cfg.SQLiteBackupDir = t.TempDir()
	cfg.SQLiteBusyTimeoutMs = 5000
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("valid persistence settings rejected: %v", err)
	}

	cfg.PersistenceBackend = "unknown"
	if err := validateConfig(cfg); err == nil {
		t.Fatal("unsupported persistence backend was accepted")
	}

	cfg.PersistenceBackend = "file"
	cfg.SQLiteBusyTimeoutMs = 0
	if err := validateConfig(cfg); err == nil {
		t.Fatal("invalid SQLite busy timeout was accepted")
	}
}

func TestChangedScopesDetectsStorageSettings(t *testing.T) {
	before := cloneConfig(GetConfig())
	after := cloneConfig(before)
	after.PersistenceBackend = "sqlite"
	if scopes := changedScopes(before, after); !containsScope(scopes, ScopeStorage) {
		t.Fatalf("storage change did not produce storage scope: %v", scopes)
	}
}

func TestRuntimeReloadRejectsStorageChangesBeforeCallbacks(t *testing.T) {
	for _, key := range []string{"PERSISTENCE_BACKEND", "DATA_DIR", "SQLITE_PATH", "SQLITE_BACKUP_DIR", "SQLITE_BUSY_TIMEOUT_MS"} {
		t.Run(key, func(t *testing.T) {
			setupRuntimeReloadTest(t)
			before := GetConfig()
			baseline := map[string]string{
				"PERSISTENCE_BACKEND":    before.PersistenceBackend,
				"DATA_DIR":               before.DataDir,
				"SQLITE_PATH":            before.SQLitePath,
				"SQLITE_BACKUP_DIR":      before.SQLiteBackupDir,
				"SQLITE_BUSY_TIMEOUT_MS": strconv.Itoa(before.SQLiteBusyTimeoutMs),
			}
			for name, value := range baseline {
				t.Setenv(name, value)
			}
			changed := baseline[key] + "-changed"
			if key == "PERSISTENCE_BACKEND" {
				changed = "sqlite"
				if before.PersistenceBackend == "sqlite" {
					changed = "file"
				}
			}
			if key == "SQLITE_BUSY_TIMEOUT_MS" {
				changed = strconv.Itoa(before.SQLiteBusyTimeoutMs + 1)
			}
			t.Setenv(key, changed)
			clearReloadCallbacks()
			t.Cleanup(clearReloadCallbacks)
			called := false
			RegisterReloadCallback(ScopeStorage, func(ReloadChange) error { called = true; return nil })
			err := ReloadRuntimeConfig()
			if err == nil || !strings.Contains(err.Error(), "require a process restart") {
				t.Fatalf("storage change accepted: %v", err)
			}
			if GetConfig() != before || GetConfig().Version != before.Version || called {
				t.Fatal("failed storage reload changed live configuration or ran callbacks")
			}
		})
	}
}

func TestRuntimeReloadPreservesConfiguredSQLitePathsWithoutOverrides(t *testing.T) {
	setupRuntimeReloadTest(t)
	previous := GetConfig()
	configMu.Lock()
	config = cloneConfig(previous)
	config.SQLitePath = filepath.Join(t.TempDir(), "custom.sqlite")
	config.SQLiteBackupDir = t.TempDir()
	expectedPath, expectedBackup := config.SQLitePath, config.SQLiteBackupDir
	configMu.Unlock()
	t.Cleanup(func() { configMu.Lock(); config = previous; configMu.Unlock() })
	t.Setenv("SQLITE_PATH", "")
	t.Setenv("SQLITE_BACKUP_DIR", "")
	if err := ReloadRuntimeConfig(); err != nil {
		t.Fatalf("unrelated reload: %v", err)
	}
	if GetConfig().SQLitePath != expectedPath || GetConfig().SQLiteBackupDir != expectedBackup {
		t.Fatal("reload reset explicitly configured storage paths")
	}
}

func TestTaskModelsUseDedicatedOverridesAndMainModelFallback(t *testing.T) {
	cfg := GetConfig()
	previousModel := cfg.AiModel
	previousReply := cfg.AiReplyModel
	previousMemory := cfg.AiMemoryModel
	previousPlanner := cfg.AiPlannerModel
	t.Cleanup(func() {
		cfg.AiModel = previousModel
		cfg.AiReplyModel = previousReply
		cfg.AiMemoryModel = previousMemory
		cfg.AiPlannerModel = previousPlanner
	})
	cfg.AiModel = "main-model"
	t.Setenv("AI_REPLY_MODEL", "reply-model")
	t.Setenv("AI_MEMORY_MODEL", "")
	t.Setenv("AI_PLANNER_MODEL", "planner-model")
	applyTaskModelsFromEnv()
	if cfg.AiReplyModel != "reply-model" || cfg.AiMemoryModel != "main-model" || cfg.AiPlannerModel != "planner-model" {
		t.Fatalf("unexpected task model routing: reply=%q memory=%q planner=%q", cfg.AiReplyModel, cfg.AiMemoryModel, cfg.AiPlannerModel)
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
