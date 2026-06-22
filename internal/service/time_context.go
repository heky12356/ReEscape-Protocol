package service

import (
	"fmt"
	"strings"
	"time"

	"project-yume/internal/config"
)

const defaultTimeContextFormat = "2006-01-02 15:04:05"

func BuildTimeContext(referenceTime time.Time) string {
	cfg := config.GetConfig()
	if !cfg.EnableTimeContext {
		return ""
	}

	if referenceTime.IsZero() {
		referenceTime = time.Now()
	}

	location, locationName := resolveTimeContextLocation(cfg.TimeContextTimezone)
	localTime := referenceTime.In(location)

	layout := strings.TrimSpace(cfg.TimeContextFormat)
	if layout == "" {
		layout = defaultTimeContextFormat
	}

	lines := []string{
		fmt.Sprintf("当前本地时间：%s", localTime.Format(layout)),
		fmt.Sprintf("当前时区：%s", locationName),
		fmt.Sprintf("今天是：%s，%s，%s", localTime.Format("2006-01-02"), chineseWeekday(localTime.Weekday()), describeDayPeriod(localTime.Hour())),
		"当用户提到今天、明天、昨天、昨晚、周末、刚刚、现在等相对时间时，请以上述时间为准理解，不要自行假设日期。",
	}

	return "【时间上下文】\n" + strings.Join(lines, "\n")
}

func resolveTimeContextLocation(raw string) (*time.Location, string) {
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

func chineseWeekday(day time.Weekday) string {
	switch day {
	case time.Monday:
		return "星期一"
	case time.Tuesday:
		return "星期二"
	case time.Wednesday:
		return "星期三"
	case time.Thursday:
		return "星期四"
	case time.Friday:
		return "星期五"
	case time.Saturday:
		return "星期六"
	default:
		return "星期日"
	}
}

func describeDayPeriod(hour int) string {
	switch {
	case hour >= 0 && hour < 5:
		return "凌晨"
	case hour < 9:
		return "早上"
	case hour < 12:
		return "上午"
	case hour < 14:
		return "中午"
	case hour < 18:
		return "下午"
	case hour < 23:
		return "晚上"
	default:
		return "深夜"
	}
}
