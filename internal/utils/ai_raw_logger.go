package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type aiRawLogger struct {
	mu          sync.Mutex
	enabled     bool
	logDir      string
	currentDate string
	file        *os.File
}

type AIRawLogEntry struct {
	Timestamp string      `json:"timestamp"`
	RequestID string      `json:"request_id"`
	Kind      string      `json:"kind"`
	Phase     string      `json:"phase"`
	Attempt   int         `json:"attempt,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Error     string      `json:"error,omitempty"`
}

var defaultAIRawLogger aiRawLogger

func ConfigureAIRawLogger(enabled bool, logDir string) error {
	defaultAIRawLogger.mu.Lock()
	defer defaultAIRawLogger.mu.Unlock()

	defaultAIRawLogger.enabled = enabled
	defaultAIRawLogger.logDir = strings.TrimSpace(logDir)

	if defaultAIRawLogger.file != nil {
		_ = defaultAIRawLogger.file.Close()
		defaultAIRawLogger.file = nil
		defaultAIRawLogger.currentDate = ""
	}

	if !enabled {
		return nil
	}

	file, currentDate, err := openAIRawLogFile(defaultAIRawLogger.logDir, time.Now())
	if err != nil {
		return err
	}
	defaultAIRawLogger.file = file
	defaultAIRawLogger.currentDate = currentDate
	return nil
}

func LogAIRaw(kind, phase, requestID string, attempt int, payload interface{}, err error) {
	defaultAIRawLogger.write(AIRawLogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		RequestID: strings.TrimSpace(requestID),
		Kind:      strings.TrimSpace(kind),
		Phase:     strings.TrimSpace(phase),
		Attempt:   attempt,
		Payload:   payload,
		Error:     stringifyError(err),
	})
}

func (l *aiRawLogger) write(entry AIRawLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.enabled {
		return
	}

	now := time.Now()
	if l.file == nil || l.currentDate != now.Format("2006-01-02") {
		if l.file != nil {
			_ = l.file.Close()
		}
		file, currentDate, err := openAIRawLogFile(l.logDir, now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open ai raw log failed: %v\n", err)
			return
		}
		l.file = file
		l.currentDate = currentDate
	}

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal ai raw log failed: %v\n", err)
		return
	}
	if _, err := l.file.Write(append(data, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "write ai raw log failed: %v\n", err)
	}
}

func openAIRawLogFile(logDir string, now time.Time) (*os.File, string, error) {
	resolvedDir := strings.TrimSpace(logDir)
	if resolvedDir == "" {
		resolvedDir = "./logs"
	}
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, "", err
	}

	currentDate := now.Format("2006-01-02")
	filename := fmt.Sprintf("ai_raw_%s.log", currentDate)
	fullPath := filepath.Join(resolvedDir, filename)
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return nil, "", err
	}
	return file, currentDate, nil
}

func stringifyError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
