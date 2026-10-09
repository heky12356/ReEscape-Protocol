package connect

import (
	"context"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"project-yume/internal/config"
	"project-yume/internal/utils"
)

var connected atomic.Bool

func SetConnected(value bool) { connected.Store(value) }

func IsConnected() bool { return connected.Load() }

func Init(host string) (*websocket.Conn, error) {
	timeout := time.Duration(config.GetConfig().OneBotDialTimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	u := url.URL{Scheme: "ws", Host: host, Path: "/ws"}
	utils.Info("连接到 %s", u.String())

	header := http.Header{}
	header.Set("Authorization", "Bearer "+config.GetConfig().Token)

	// 建立 WebSocket 连接
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = timeout
	c, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		utils.Error("连接失败: %v", err)
		return nil, err
	}

	ensureOutboundWriter(c)
	return c, nil
}

// StartKeepalive keeps a connected OneBot session alive and turns a missing
// pong into a read timeout observed by the receiver.
func StartKeepalive(ctx context.Context, conn *websocket.Conn) {
	if conn == nil {
		return
	}
	cfg := config.GetConfig()
	interval := time.Duration(cfg.OneBotHeartbeatIntervalMs) * time.Millisecond
	readTimeout := time.Duration(cfg.OneBotReadTimeoutMs) * time.Millisecond
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if readTimeout <= 0 {
		readTimeout = 3 * interval
	}
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readTimeout))
	})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := WriteMessage(conn, websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()
}
