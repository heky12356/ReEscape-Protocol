package app

import (
	"context"
	"math"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/metrics"
	"project-yume/internal/utils"
)

// RunConnectionSupervisor owns the OneBot connection lifecycle. A failed
// connection ends its worker group; the next attempt always dials a new socket.
func RunConnectionSupervisor(ctx context.Context, components *Components) {
	if components == nil {
		return
	}
	cfg := config.GetConfig()
	host := cfg.Hostadd + ":" + cfg.WsPort
	initial := durationMs(cfg.OneBotReconnectInitialMs, time.Second)
	maximum := durationMs(cfg.OneBotReconnectMaxMs, 30*time.Second)
	if maximum < initial {
		maximum = initial
	}
	backoff := initial

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return
		}
		conn, err := connect.Init(host)
		if err != nil {
			utils.Error("OneBot 连接失败(attempt=%d): %v", attempt+1, err)
			waitContext(ctx, backoff)
			backoff = time.Duration(math.Min(float64(maximum), float64(backoff)*2))
			continue
		}
		backoff = initial
		utils.Info("OneBot 连接成功: %s", host)
		metricsConnected(true)

		sessionCtx, cancel := context.WithCancel(ctx)
		receiverDone := StartWorkers(sessionCtx, conn, components)
		var receiveErr error
		select {
		case receiveErr = <-receiverDone:
			if receiveErr != nil && sessionCtx.Err() == nil {
				utils.Warn("OneBot 连接会话结束: %v", receiveErr)
			}
		case <-ctx.Done():
		}
		cancel()
		_ = connect.Close(conn)
		metricsConnected(false)
		if ctx.Err() != nil {
			return
		}
		utils.Warn("OneBot 连接已断开，%s 后重连", backoff)
		waitContext(ctx, backoff)
		backoff = time.Duration(math.Min(float64(maximum), float64(backoff)*2))
	}
}

func durationMs(value int, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return time.Duration(value) * time.Millisecond
}

func waitContext(ctx context.Context, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func metricsConnected(connected bool) {
	connect.SetConnected(connected)
	value := "false"
	if connected {
		value = "true"
	}
	metrics.IncCounter("bot_onebot_connection_events_total", "OneBot connection lifecycle events.", map[string]string{"state": value})
}
