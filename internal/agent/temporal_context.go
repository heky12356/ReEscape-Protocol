package agent

import (
	"fmt"
	"strings"
	"time"

	"project-yume/internal/config"
)

const defaultTemporalContextFormat = "2006-01-02 15:04:05"

func buildTemporalContext(turn *TurnContext) string {
	cfg := config.GetConfig()
	if turn == nil || !cfg.EnableTimeContext {
		return ""
	}

	location, _ := resolveTemporalLocation(cfg.TimeContextTimezone)
	layout := strings.TrimSpace(cfg.TimeContextFormat)
	if layout == "" {
		layout = defaultTemporalContextFormat
	}

	referenceTime := turn.ReferenceTime()
	if referenceTime.IsZero() {
		referenceTime = time.Now()
	}
	eventTime := turn.EndedAt()
	if eventTime.IsZero() {
		eventTime = referenceTime
	}

	lines := []string{
		"【Temporal Context】",
		fmt.Sprintf("当前本地时间：%s", referenceTime.In(location).Format(layout)),
	}

	if turn.Trigger() == string(TriggerProactive) {
		lines = append(lines, fmt.Sprintf("当前主动触发时间：%s", eventTime.In(location).Format(layout)))
	} else {
		lines = append(lines, fmt.Sprintf("当前用户消息发送于：%s", eventTime.In(location).Format(layout)))
		lines = append(lines,
			fmt.Sprintf("距离上次用户消息：%s", formatTemporalHistoryDistance(eventTime, turn.PreviousUserMessageAt())),
			fmt.Sprintf("距离上次助手回复：%s", formatTemporalHistoryDistance(eventTime, turn.PreviousAssistantMessageAt())),
		)
	}

	if aggregationLine := buildAggregationLine(turn); aggregationLine != "" {
		lines = append(lines, aggregationLine)
	}
	if shouldWarnTemporalDiscontinuity(eventTime, turn.PreviousInteractionAt()) {
		lines = append(lines, "提示：当前对话与上一轮不连续，请按跨时段延续理解。")
	}
	lines = append(lines, "当用户提到今天、明天、昨天、刚刚、现在、昨晚等相对时间时，以上述时间为准理解。")

	return strings.Join(lines, "\n")
}

func resolveTemporalLocation(raw string) (*time.Location, string) {
	timezone := strings.TrimSpace(raw)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Local, time.Local.String()
	}
	return location, timezone
}

func formatTemporalHistoryDistance(current, previous time.Time) string {
	if current.IsZero() || previous.IsZero() {
		return "无历史记录。"
	}
	if previous.After(current) {
		return "无历史记录。"
	}
	return formatTemporalDuration(current.Sub(previous)) + "。"
}

func buildAggregationLine(turn *TurnContext) string {
	count := turn.SegmentCount()
	if count <= 0 {
		count = len(turn.RawSegments())
	}
	if !turn.Aggregated() && count <= 1 {
		return ""
	}
	if count <= 0 {
		count = 1
	}

	span := turn.EndedAt().Sub(turn.StartedAt())
	if span < 0 {
		span = 0
	}
	return fmt.Sprintf("本轮由 %d 条连续消息聚合，跨度 %s。", count, formatTemporalDuration(span))
}

func shouldWarnTemporalDiscontinuity(current, previous time.Time) bool {
	if current.IsZero() || previous.IsZero() || previous.After(current) {
		return false
	}
	return current.Sub(previous) >= 6*time.Hour
}

func formatTemporalDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	totalSeconds := int64(duration.Round(time.Second).Seconds())
	if totalSeconds < 60 {
		return fmt.Sprintf("%d 秒", totalSeconds)
	}

	totalMinutes := totalSeconds / 60
	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%d 天", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%d 小时", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d 分钟", minutes))
	}
	return strings.Join(parts, " ")
}
