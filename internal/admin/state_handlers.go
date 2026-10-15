package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"project-yume/internal/storage"

	"github.com/gin-gonic/gin"
)

func (s *server) handlePersistence(c *gin.Context) {
	if s.persistence == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "persistence is not initialized"})
		return
	}
	info, err := s.persistence.Info(c.Request.Context())
	if err != nil {
		stateQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func (s *server) handleSQLiteBackup(c *gin.Context) {
	if s.stateQueryStore(c) == nil {
		return
	}
	manager, err := s.transferManager()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	job, err := manager.StartSQLiteBackup(s.persistence)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, job)
}

func (s *server) stateQueryStore(c *gin.Context) *storage.SQLiteQueryStore {
	if s.persistence == nil || !s.persistence.Status().Ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "persistence is not ready"})
		return nil
	}
	if s.persistence.Status().Backend != storage.BackendSQLite {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "SQLite read models are unavailable", "backend": s.persistence.Status().Backend})
		return nil
	}
	return storage.NewSQLiteQueryStore(s.persistence.Database())
}

func (s *server) handleQueryState(c *gin.Context) {
	query := s.stateQueryStore(c)
	if query == nil {
		return
	}
	options := storage.QueryOptions{Limit: storage.DefaultQueryLimit, SessionID: c.Query("session_id"), Status: c.Query("status"), Cursor: c.Query("cursor")}
	allowed := map[string]bool{"user_id": true, "session_id": true, "status": true, "limit": true, "cursor": true}
	for key, values := range c.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown, repeated or empty query parameter: " + key})
			return
		}
	}
	if raw := c.Query("user_id"); raw != "" {
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id must be a non-negative integer"})
			return
		}
		options.UserID = parsed
	}
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || parsed < 1 || parsed > storage.MaxQueryLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 200"})
			return
		}
		options.Limit = parsed
	}
	page, err := query.Page(c.Request.Context(), c.Param("kind"), options)
	if err != nil {
		stateQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (s *server) handleStateDetail(c *gin.Context) {
	query := s.stateQueryStore(c)
	if query == nil {
		return
	}
	if len(c.Request.URL.Query()) != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "detail does not accept query filters"})
		return
	}
	record, err := query.Get(c.Request.Context(), c.Param("kind"), c.Param("id"))
	if err != nil {
		stateQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, record)
}

func (s *server) handleRebuildDeliveries(c *gin.Context) {
	if s.stateQueryStore(c) == nil {
		return
	}
	if err := storage.RebuildDeliveryReadModels(c.Request.Context(), s.persistence.Database()); err != nil {
		stateQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "completed"})
}

func stateQueryError(c *gin.Context, err error) {
	var invalid *storage.QueryValidationError
	switch {
	case errors.As(err, &invalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, storage.ErrReadModelChanged):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "read_model_changed"})
	case errors.Is(err, sql.ErrNoRows):
		c.JSON(http.StatusNotFound, gin.H{"error": "record not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "state query failed"})
	}
}

func (s *server) handleSessionSummary(c *gin.Context) {
	query := s.stateQueryStore(c)
	if query == nil {
		return
	}
	if len(c.Request.URL.Query()) != 0 {
		c.JSON(400, gin.H{"error": "summary does not accept query filters"})
		return
	}
	record, err := query.SessionSummary(c.Request.Context(), c.Param("id"))
	if err != nil {
		stateQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, record)
}

func (s *server) handleConversationMessages(c *gin.Context) {
	query := s.stateQueryStore(c)
	if query == nil {
		return
	}
	for key, values := range c.Request.URL.Query() {
		if (key != "limit" && key != "cursor") || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			c.JSON(400, gin.H{"error": "unknown, repeated or empty query parameter: " + key})
			return
		}
	}
	limit := storage.DefaultQueryLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > storage.MaxQueryLimit {
			c.JSON(400, gin.H{"error": "limit must be between 1 and 200"})
			return
		}
		limit = parsed
	}
	page, err := query.Messages(c.Request.Context(), c.Param("id"), limit, c.Query("cursor"))
	if err != nil {
		stateQueryError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}
