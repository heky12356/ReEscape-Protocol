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
			claimed, nextAt := scheduler.TryClaimDue(sessionID, now)
			due := !nextAt.IsZero() && !now.Before(nextAt)
			utils.Info("自然调度检查: next=%s due=%t claimed=%t", nextAt.Format(time.RFC3339), due, claimed)
			if !claimed {
				continue
			}

			appendTurnGateEvent(ctx, eventStore, "proactive_claimed", sessionID, targetUserID, map[string]any{
				"next_at": nextAt.Format(time.RFC3339),
			})
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
		sentAt, sent, err := sendScheduledAgentMessage(ctx, c, scheduler, agentRuntime, eventStore, sessionID, targetUserID)
		if err == nil {
			if sent {
				scheduler.CompleteClaim(sessionID, sentAt)
			} else {
				scheduler.ReleaseClaim(sessionID)
			}
			return nil
		}

		if !scheduler.IsIdleForProactive(sessionID, time.Now()) {
			scheduler.ReleaseClaim(sessionID)
			appendTurnGateEvent(ctx, eventStore, "proactive_skipped", sessionID, targetUserID, map[string]any{
				"reason": "user_turn_pending_after_agent_error",
				"error":  err.Error(),
			})
			utils.Info("主动消息 fallback 前发现用户 turn pending，跳过 legacy 主动回复")
			return nil
		}

		utils.Warn("React proactive agent failed, fallback to legacy scheduler: %v", err)
	}
	if !scheduler.IsIdleForProactive(sessionID, time.Now()) {
		scheduler.ReleaseClaim(sessionID)
		appendTurnGateEvent(ctx, eventStore, "proactive_skipped", sessionID, targetUserID, map[string]any{
			"reason": "user_turn_pending_before_legacy_send",
		})
		utils.Info("主动消息发送前发现用户 turn pending，跳过 proactive")
		return nil
	}
	err := legacyscheduler.SendScheduledMessage(c, scheduler, sessionID, targetUserID)
	if err != nil {
		scheduler.ReleaseClaim(sessionID)
		return err
	}
	scheduler.CompleteClaim(sessionID, time.Now())
	return nil
}

func sendScheduledAgentMessage(ctx context.Context, c *websocket.Conn, scheduler *scheduler.NaturalScheduler,
	agentRuntime *agent.Runtime, eventStore eventlog.Store, sessionID string, targetUserID int64,
) (time.Time, bool, error) {
	state.GetManager().EnsureSession(sessionID, targetUserID, 0, 1)
	now := time.Now()
	if !state.GetManager().HasActiveProactiveClaim(sessionID, now) {
		utils.Info("主动消息发送前检查 claim 已失效")
		return time.Time{}, false, nil
	}
	if !scheduler.IsIdleForProactive(sessionID, now) {
		appendTurnGateEvent(ctx, eventStore, "proactive_skipped", sessionID, targetUserID, map[string]any{
			"reason": "user_turn_pending_before_agent_run",
		})
		utils.Info("主动消息运行前发现用户 turn pending，跳过 proactive")
		return time.Time{}, false, nil
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
		return time.Time{}, false, err
	}

	reply := strings.TrimSpace(result.FinalReply.Content)
	if !result.ShouldSend || reply == "" {
		if eventStore != nil {
			if err := eventStore.Append(ctx, result.Events...); err != nil {
				utils.Warn("append proactive agent events failed: %v", err)
			}
		}
		return time.Time{}, false, fmt.Errorf("agent proactive reply is empty")
	}
	if !scheduler.IsIdleForProactive(sessionID, time.Now()) {
		result.Events = append(result.Events, eventlog.Event{
			Type:      "proactive_skipped",
			SessionID: sessionID,
			UserID:    targetUserID,
			Actor:     "runtime",
			CreatedAt: time.Now(),
			Data: map[string]any{
				"reason": "user_turn_pending_before_send",
			},
		})
		if eventStore != nil {
			if err := eventStore.Append(ctx, result.Events...); err != nil {
				utils.Warn("append proactive agent events failed: %v", err)
			}
		}
		utils.Info("主动消息发送前发现用户 turn pending，跳过 proactive")
		return time.Time{}, false, nil
	}
	if err := service.SendMsg(c, targetUserID, reply); err != nil {
		return time.Time{}, false, err
	}

	sentAt := time.Now()
	transcript := service.BuildAssistantTranscript(reply)
	state.GetManager().RecordAssistantTurn(sessionID, transcript, sentAt, true)
	state.GetManager().UpdateLastReplyMode(sessionID, "proactive")
	var next time.Time
	if result.ScheduleManaged {
		next = state.GetManager().GetNextScheduledAt(sessionID)
	} else {
		next = scheduler.RescheduleFrom(sessionID, sentAt)
	}
	nextAt := ""
	if !next.IsZero() {
		nextAt = next.Format(time.RFC3339)
	}
	result.Events = append(result.Events, eventlog.Event{
		Type:      "reply_sent",
		SessionID: sessionID,
		UserID:    targetUserID,
		Actor:     "runtime",
		Message:   transcript,
		CreatedAt: sentAt,
		Data: map[string]any{
			"trigger": "proactive",
			"next_at": nextAt,
		},
	})
	if eventStore != nil {
		if err := eventStore.Append(ctx, result.Events...); err != nil {
			utils.Warn("append proactive agent events failed: %v", err)
		}
	}
	utils.Info("Agent 主动消息已发送，下一次主动触达时间: %s", nextAt)
	return sentAt, true, nil
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
