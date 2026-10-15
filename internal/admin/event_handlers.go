package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/eventlog"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func parseEventQuery(c *gin.Context) (eventlog.ReplayOptions, string, error) {
	options := eventlog.ReplayOptions{Limit: 200}
	filters := map[string]*string{"type": &options.Filters.Type, "request_id": &options.Filters.RequestID, "turn_id": &options.Filters.TurnID, "session_id": &options.Filters.SessionID, "intent_id": &options.Filters.IntentID, "delivery_id": &options.Filters.DeliveryID, "tool": &options.Filters.Tool}
	for key, values := range c.Request.URL.Query() {
		if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return options, "", fmt.Errorf("repeated or empty event parameter: %s", key)
		}
		value := strings.TrimSpace(values[0])
		if target, ok := filters[key]; ok {
			*target = value
			continue
		}
		switch key {
		case "after":
			after, err := strconv.ParseInt(value, 10, 64)
			if err != nil || after < 0 {
				return options, "", fmt.Errorf("after must be a non-negative event cursor")
			}
			options.After = &after
		case "limit":
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 1000 {
				return options, "", fmt.Errorf("limit must be between 1 and 1000")
			}
			options.Limit = limit
		case "stream_id":
			if len(value) > 200 {
				return options, "", fmt.Errorf("invalid stream_id")
			}
		default:
			return options, "", fmt.Errorf("unknown event parameter: %s", key)
		}
	}
	if c.Query("stream_id") != "" && options.After == nil {
		return options, "", fmt.Errorf("stream_id requires after")
	}
	return options, strings.TrimSpace(c.Query("stream_id")), nil
}

func (s *server) eventPage(c *gin.Context, store eventlog.ReplayStore, options eventlog.ReplayOptions, streamID string) (eventLogResponse, error) {
	page, err := store.Replay(c.Request.Context(), options)
	if err != nil {
		return eventLogResponse{}, err
	}
	response := eventLogResponse{Events: page.Events, EventCursors: []string{}, Cursor: strconv.FormatInt(page.Cursor, 10), EarliestCursor: strconv.FormatInt(page.Earliest, 10), LatestCursor: strconv.FormatInt(page.Latest, 10), HasMore: page.HasMore, StreamID: s.eventStreamID, Reason: page.Reason}
	if streamID != "" && streamID != s.eventStreamID {
		response.Reason = "stream_changed"
		response.Events = []eventlog.Event{}
		response.HasMore = false
	}
	for _, event := range response.Events {
		response.EventCursors = append(response.EventCursors, strconv.FormatInt(event.Cursor, 10))
	}
	return response, nil
}

func (s *server) handleEvents(c *gin.Context) {
	setNoCacheHeaders(c)
	options, streamID, err := parseEventQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	store, ok := eventlog.Default().(eventlog.ReplayStore)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "atomic event replay is unavailable"})
		return
	}
	page, err := s.eventPage(c, store, options, streamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "event query failed"})
		return
	}
	status := http.StatusOK
	if page.Reason != "" {
		status = http.StatusConflict
	}
	c.JSON(status, page)
}

func (s *server) handleEventStream(c *gin.Context) {
	options, streamID, err := parseEventQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	store, ok := eventlog.Default().(eventlog.ReplayStore)
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "atomic event replay is unavailable"})
		return
	}
	if options.Limit > 200 {
		options.Limit = 200
	}
	if options.After == nil {
		baseline, err := store.Replay(c.Request.Context(), eventlog.ReplayOptions{Limit: 1})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "event query failed"})
			return
		}
		options.After = &baseline.Cursor
	}
	upgrader := websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 4096, CheckOrigin: func(request *http.Request) bool {
		origin := strings.TrimSpace(request.Header.Get("Origin"))
		return origin == "" || allowedOrigin(origin, config.GetConfig().AdminCORSOrigins) || origin == "http://"+request.Host || origin == "https://"+request.Host
	}}
	connection, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(1024)
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()
	write := func(value any) error {
		if err := connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		return connection.WriteJSON(value)
	}
	unauthorized := func() bool {
		if s.eventStreamAuthorized(c) {
			return false
		}
		_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "authentication expired"), time.Now().Add(time.Second))
		return true
	}
	writeBatch := func() (bool, error) {
		if unauthorized() {
			return false, fmt.Errorf("authentication expired")
		}
		page, err := s.eventPage(c, store, options, streamID)
		if err != nil {
			return false, err
		}
		if page.Reason != "" {
			kind := "reset"
			if page.Reason == "retention_gap" {
				kind = "gap"
			}
			_ = write(map[string]any{"type": kind, "reason": page.Reason, "streamId": page.StreamID, "earliestCursor": page.EarliestCursor, "latestCursor": page.LatestCursor})
			return false, fmt.Errorf("event history changed")
		}
		for i, event := range page.Events {
			if err := write(map[string]any{"type": "event", "event": event, "cursor": page.EventCursors[i], "streamId": page.StreamID}); err != nil {
				return false, err
			}
		}
		if err := write(map[string]any{"type": "checkpoint", "cursor": page.Cursor, "streamId": page.StreamID, "hasMore": page.HasMore}); err != nil {
			return false, err
		}
		after, err := strconv.ParseInt(page.Cursor, 10, 64)
		if err != nil {
			return false, err
		}
		options.After = &after
		return page.HasMore, nil
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		// Bound each burst for cancellation/auth checks, then continue immediately
		// when there is backlog. Cursor checkpoints advance even without matches.
		more, err := writeBatch()
		if err != nil {
			return
		}
		if more {
			select {
			case <-closed:
				return
			case <-c.Request.Context().Done():
				return
			default:
				continue
			}
		}
		select {
		case <-closed:
			return
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *server) eventStreamAuthorized(c *gin.Context) bool {
	secret := config.GetConfig().AdminAPIKey
	if method, _ := c.Get("admin_auth_method"); method == "session" {
		cookie, err := c.Request.Cookie(adminSessionCookie)
		return err == nil && s.sessions.valid(cookie.Value, time.Now(), secret)
	}
	provided := strings.TrimSpace(c.GetHeader("X-API-Key"))
	if provided == "" {
		scheme, value, ok := strings.Cut(c.GetHeader("Authorization"), " ")
		if ok && strings.EqualFold(scheme, "Bearer") {
			provided = value
		}
	}
	return constantTimeSecretEqual(provided, secret)
}
