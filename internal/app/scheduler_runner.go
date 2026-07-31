package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/agent"
	"project-yume/internal/config"
	"project-yume/internal/eventlog"
	legacyscheduler "project-yume/internal/legacy/scheduler"
	"project-yume/internal/scheduler"
	"project-yume/internal/service"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

func startScheduler(c *websocket.Conn, scheduler *scheduler.NaturalScheduler, agentRuntime *agent.Runtime, eventStore eventlog.Store,
	ctx context.Context, sessionID string, targetUserID int64,
) {
	time.Sleep(time.Second)
	ticker := time.NewTicker(scheduler.SweepInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			utils.Info("定时器已停止")
			return
		case <-ticker.C:
			now := time.Now()
			shouldSend, nextAt := scheduler.ShouldSendNow(sessionID, now)
			utils.Info("自然调度检查: next=%s due=%t", nextAt.Format(time.RFC3339), shouldSend)
			if !shouldSend {
				continue
			}

			utils.Info("定时器触发")
			err := sendScheduledTurn(ctx, c, scheduler, agentRuntime, eventStore, sessionID, targetUserID)
			if err != nil {
				utils.Error("定时消息发送失败: %v", err)
			} else {
				utils.Info("定时器触发成功")
			}
		}
	}
}

func sendScheduledTurn(ctx context.Context, c *websocket.Conn, scheduler *scheduler.NaturalScheduler,
	agentRuntime *agent.Runtime, eventStore eventlog.Store, sessionID string, targetUserID int64,
) error {
	if config.GetConfig().EnableReactAgent && agentRuntime != nil {
		if err := sendScheduledAgentMessage(ctx, c, scheduler, agentRuntime, eventStore, sessionID, targetUserID); err != nil {
			utils.Warn("React proactive agent failed, fallback to legacy scheduler: %v", err)
		} else {
			return nil
		}
	}
	return legacyscheduler.SendScheduledMessage(c, scheduler, sessionID, targetUserID)
}

func sendScheduledAgentMessage(ctx context.Context, c *websocket.Conn, scheduler *scheduler.NaturalScheduler,
	agentRuntime *agent.Runtime, eventStore eventlog.Store, sessionID string, targetUserID int64,
) error {
	state.GetManager().EnsureSession(sessionID, targetUserID, 0, 1)
	if shouldSend, nextAt := scheduler.ShouldSendNow(sessionID, time.Now()); !shouldSend {
		utils.Info("主动消息发送前检查未到时间, next=%s", nextAt.Format(time.RFC3339))
		return nil
	}

	turn := agent.NewTurnContext(agent.TurnInput{
		RequestID:     utils.NewRequestID("proactive"),
		SessionID:     sessionID,
		UserID:        targetUserID,
		ChatType:      1,
		ReferenceTime: time.Now(),
		Trigger:       agent.TriggerProactive,
		Actor:         agent.ActorScheduler,
	})
	result, err := agentRuntime.RunTurn(ctx, turn)
	if err != nil {
		return err
	}

	reply := strings.TrimSpace(result.FinalReply.Content)
	if !result.ShouldSend || reply == "" {
		return fmt.Errorf("agent proactive reply is empty")
	}
	if err := service.SendMsg(c, targetUserID, reply); err != nil {
		return err
	}

	sentAt := time.Now()
	transcript := service.BuildAssistantTranscript(reply)
	state.GetManager().RecordAssistantTurn(sessionID, transcript, sentAt, true)
	state.GetManager().UpdateLastReplyMode(sessionID, "proactive")
	next := scheduler.RescheduleFrom(sessionID, sentAt)
	result.Events = append(result.Events, eventlog.Event{
		Type:      "reply_sent",
		SessionID: sessionID,
		UserID:    targetUserID,
		Actor:     "runtime",
		Message:   transcript,
		CreatedAt: sentAt,
		Data: map[string]any{
			"trigger": "proactive",
			"next_at": next.Format(time.RFC3339),
		},
	})
	if eventStore != nil {
		if err := eventStore.Append(ctx, result.Events...); err != nil {
			utils.Warn("append proactive agent events failed: %v", err)
		}
	}
	utils.Info("Agent 主动消息已发送，下一次主动触达时间: %s", next.Format(time.RFC3339))
	return nil
}

func StartStatusMonitor(ctx context.Context, sessionID string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			utils.Info("状态监控器已停止")
			return
		case <-ticker.C:
			sm := state.GetManager()
			utils.Info("当前状态(%s): %v, 上次回复: %v, 上次互动: %v, 下次主动触达: %s",
				sessionID,
				sm.GetState(sessionID),
				sm.GetTimeSinceLastReply(sessionID),
				sm.GetTimeSinceLastInteraction(sessionID),
				sm.GetNextScheduledAt(sessionID).Format(time.RFC3339),
			)
		}
	}
}
