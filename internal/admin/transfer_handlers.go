package admin

import (
	"errors"
	"net/http"

	"project-yume/internal/storage"

	"github.com/gin-gonic/gin"
)

func transferError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, storage.ErrTransferBusy):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "transfer_busy"})
	case errors.Is(err, storage.ErrTransferClosed):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error(), "code": "transfer_closed"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func (s *server) handleTransfers(c *gin.Context) {
	setNoCacheHeaders(c)
	manager, err := s.transferManager()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, struct {
		storage.TransferOverview
		Backend     storage.BackendKind `json:"backend"`
		RestoreMode string              `json:"restoreMode"`
		Retention   string              `json:"retention"`
	}{manager.Overview(), s.persistence.Status().Backend, "offline", "manual"})
}

// Online requests never enter an offline restore implementation. Managers and
// flush workers still own live state; the CLI must acquire their process lease.
func (s *server) handleRestoreTransfer(c *gin.Context) {
	manager, err := s.transferManager()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	job, ok := manager.Get(c.Param("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "transfer job not found"})
		return
	}
	c.JSON(http.StatusConflict, gin.H{"error": "online restore is unavailable; stop the bot, wait for its final flush, then use datactl with the configured data/sqlite/backup paths", "code": "offline_restore_required", "applied": false, "kind": job.Kind, "restoreMode": "offline"})
}
