package eventlog

import "context"

// ReplayStore returns a bounded scan and its cursor/bounds from one snapshot.
// Filters affect visible events, never the scan cursor or hasMore.
type ReplayStore interface {
	Store
	Replay(context.Context, ReplayOptions) (ReplayPage, error)
}

type Filters struct {
	Type, RequestID, TurnID, SessionID, IntentID, DeliveryID, Tool string
}

func (f Filters) Matches(event Event) bool {
	tool := event.ToolName
	if tool == "" {
		tool = event.Tool
	}
	return (f.Type == "" || f.Type == event.Type) &&
		(f.RequestID == "" || f.RequestID == event.RequestID) &&
		(f.TurnID == "" || f.TurnID == event.TurnID) &&
		(f.SessionID == "" || f.SessionID == event.SessionID) &&
		(f.IntentID == "" || f.IntentID == event.IntentID) &&
		(f.DeliveryID == "" || f.DeliveryID == event.DeliveryID) &&
		(f.Tool == "" || f.Tool == tool)
}

type ReplayOptions struct {
	After   *int64 // nil selects the latest window; zero explicitly replays from start
	Limit   int
	Filters Filters
}

type ReplayPage struct {
	Events                   []Event
	Cursor, Earliest, Latest int64
	HasMore                  bool
	Reason                   string
}

func replayLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func filterReplay(page *ReplayPage, filters Filters) {
	visible := make([]Event, 0, len(page.Events))
	for _, event := range page.Events {
		if filters.Matches(event) {
			visible = append(visible, event)
		}
	}
	page.Events = visible
}
