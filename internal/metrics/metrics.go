package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type sample struct {
	labels map[string]string
	value  float64
}

type metric struct {
	help    string
	samples map[string]*sample
}

type Registry struct {
	startedAt time.Time

	mu       sync.RWMutex
	counters map[string]*metric
}

var defaultRegistry = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{
		startedAt: time.Now(),
		counters:  make(map[string]*metric),
	}
}

func Default() *Registry {
	return defaultRegistry
}

func (r *Registry) StartedAt() time.Time {
	return r.startedAt
}

func IncCounter(name, help string, labels map[string]string) {
	defaultRegistry.AddCounter(name, help, 1, labels)
}

func AddCounter(name, help string, delta float64, labels map[string]string) {
	defaultRegistry.AddCounter(name, help, delta, labels)
}

func ObserveDuration(name, help string, duration time.Duration, labels map[string]string) {
	defaultRegistry.ObserveDuration(name, help, duration, labels)
}

func RenderPrometheus() string {
	return defaultRegistry.RenderPrometheus()
}

func (r *Registry) AddCounter(name, help string, delta float64, labels map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	metricEntry := r.counters[name]
	if metricEntry == nil {
		metricEntry = &metric{
			help:    help,
			samples: make(map[string]*sample),
		}
		r.counters[name] = metricEntry
	}

	key, copiedLabels := serializeLabels(labels)
	current := metricEntry.samples[key]
	if current == nil {
		current = &sample{labels: copiedLabels}
		metricEntry.samples[key] = current
	}
	current.value += delta
}

func (r *Registry) ObserveDuration(name, help string, duration time.Duration, labels map[string]string) {
	milliseconds := float64(duration.Milliseconds())
	r.AddCounter(name+"_ms_total", help+" Total observed duration in milliseconds.", milliseconds, labels)
	r.AddCounter(name+"_count", help+" Total observation count.", 1, labels)
}

func (r *Registry) RenderPrometheus() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var builder strings.Builder

	builder.WriteString("# HELP bot_uptime_seconds Process uptime in seconds.\n")
	builder.WriteString("# TYPE bot_uptime_seconds gauge\n")
	builder.WriteString(fmt.Sprintf("bot_uptime_seconds %.0f\n", time.Since(r.startedAt).Seconds()))

	names := make([]string, 0, len(r.counters))
	for name := range r.counters {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		metricEntry := r.counters[name]
		if metricEntry == nil {
			continue
		}

		builder.WriteString(fmt.Sprintf("# HELP %s %s\n", name, escapeHelp(metricEntry.help)))
		builder.WriteString(fmt.Sprintf("# TYPE %s counter\n", name))

		sampleKeys := make([]string, 0, len(metricEntry.samples))
		for key := range metricEntry.samples {
			sampleKeys = append(sampleKeys, key)
		}
		sort.Strings(sampleKeys)

		for _, sampleKey := range sampleKeys {
			sampleEntry := metricEntry.samples[sampleKey]
			if sampleEntry == nil {
				continue
			}

			if len(sampleEntry.labels) == 0 {
				builder.WriteString(fmt.Sprintf("%s %.6f\n", name, sampleEntry.value))
				continue
			}

			builder.WriteString(name)
			builder.WriteString("{")
			builder.WriteString(formatLabels(sampleEntry.labels))
			builder.WriteString("} ")
			builder.WriteString(fmt.Sprintf("%.6f\n", sampleEntry.value))
		}
	}

	return builder.String()
}

func serializeLabels(labels map[string]string) (string, map[string]string) {
	if len(labels) == 0 {
		return "", nil
	}

	keys := make([]string, 0, len(labels))
	copied := make(map[string]string, len(labels))
	for key, value := range labels {
		keys = append(keys, key)
		copied[key] = value
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+copied[key])
	}
	return strings.Join(parts, ","), copied
}

func formatLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, escapeLabelValue(labels[key])))
	}
	return strings.Join(parts, ",")
}

func escapeHelp(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}
