package agent

import (
	"time"

	"project-yume/internal/config"
)

type Budget struct {
	MaxSteps     int
	TotalTimeout time.Duration
}

func BudgetFromConfig() Budget {
	cfg := config.GetConfig()
	maxSteps := cfg.ReactMaxSteps
	if maxSteps <= 0 {
		maxSteps = 4
	}

	totalTimeout := time.Duration(cfg.ReactTotalTimeoutMs) * time.Millisecond
	if totalTimeout <= 0 {
		totalTimeout = 30 * time.Second
	}

	return Budget{
		MaxSteps:     maxSteps,
		TotalTimeout: totalTimeout,
	}
}
