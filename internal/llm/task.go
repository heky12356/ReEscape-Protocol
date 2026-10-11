package llm

import "strings"

// Task identifies the purpose of a model call independently from its provider.
type Task string

const (
	TaskReply   Task = "reply"
	TaskMemory  Task = "memory"
	TaskPlanner Task = "planner"
)

func (t Task) String() string { return string(t) }

func ParseTask(raw string) (Task, bool) {
	switch Task(strings.ToLower(strings.TrimSpace(raw))) {
	case TaskReply:
		return TaskReply, true
	case TaskMemory:
		return TaskMemory, true
	case TaskPlanner:
		return TaskPlanner, true
	default:
		return "", false
	}
}

func normalizeTask(task Task) Task {
	if parsed, ok := ParseTask(string(task)); ok {
		return parsed
	}
	return TaskReply
}
