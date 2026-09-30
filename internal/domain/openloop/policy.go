package openloop

import "strings"

func Normalize(loop OpenLoop) OpenLoop {
	loop.ID = strings.TrimSpace(loop.ID)
	loop.SessionID = strings.TrimSpace(loop.SessionID)
	loop.Description = strings.TrimSpace(loop.Description)
	loop.Status = strings.TrimSpace(loop.Status)
	if loop.Status == "" {
		loop.Status = "open"
	}
	return loop
}

func IsActionable(loop OpenLoop) bool {
	loop = Normalize(loop)
	return loop.Description != "" && loop.Status == "open"
}
