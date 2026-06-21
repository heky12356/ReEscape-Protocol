package utils

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var requestCounter atomic.Uint64

func NewRequestID(prefix string) string {
	normalizedPrefix := strings.TrimSpace(prefix)
	if normalizedPrefix == "" {
		normalizedPrefix = "req"
	}

	sequence := requestCounter.Add(1)
	return fmt.Sprintf("%s-%d-%d", normalizedPrefix, time.Now().UnixMilli(), sequence)
}
