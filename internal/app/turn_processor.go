package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"project-yume/internal/agent"
	"project-yume/internal/config"
	"project-yume/internal/eventlog"
	"project-yume/internal/handler"
	"project-yume/internal/inbound"
	"project-yume/internal/metrics"
	"project-yume/internal/model"
	"project-yume/internal/scheduler"
	"project-yume/internal/service"
	"project-yume/internal/state"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

func startMessageProcessor(c *websocket.Conn, msgChan chan model.Msg,
	pipeline *inbound.Pipeline, processor *handler.MessageProcessor, agentRuntime *agent.RuntimeHandle, eventStore eventlog.Store,
	naturalScheduler *scheduler.NaturalScheduler, ctx context.Context,
) {
	cfg := config.GetConfig()

	for {
		select {
		case <-ctx.Done():
			utils.Info("消息处理器已停止")
			return
		case msg, ok := <-msgChan:
			if !ok {
				utils.Info("消息通道已关闭")
				return
			}

			sessionID := state.BuildSessionID(msg.User_id, msg.Group_id, msg.Type)
			enrichedParts := service.EnrichMessageParts(c, msg.Parts)
			if len(enrichedParts) == 0 {
				enrichedParts = append([]model.MessagePart(nil), msg.Parts...)
			}
			startedAt := time.Unix(msg.Time, 0)
			if msg.StartTime != 0 {
				startedAt = time.Unix(msg.StartTime, 0)
			}
			endedAt := time.Unix(msg.Time, 0)
			if msg.EndTime != 0 {
				endedAt = time.Unix(msg.EndTime, 0)
			}
			messageIDs := msg.MessageIDs
			if len(messageIDs) == 0 && msg.MessageID != 0 {
				messageIDs = []int64{msg.MessageID}
			}
			rawSegments := msg.RawSegments
			if len(rawSegments) == 0 && msg.Message != "" {
				rawSegments = []string{msg.Message}
			}
			rawSegmentTimes := msg.RawSegmentTimes
			if len(rawSegmentTimes) == 0 && msg.Time != 0 {
				rawSegmentTimes = []int64{msg.Time}
			}
			messageCtx := handler.MessageContext{
				RequestID:       buildMessageRequestID(msg.MessageID),
				SessionID:       sessionID,
				UserID:          msg.User_id,
				GroupID:         msg.Group_id,
				ChatType:        msg.Type,
				MessageID:       msg.MessageID,
				MessageIDs:      messageIDs,
				RawSegments:     rawSegments,
				RawSegmentTimes: rawSegmentTimes,
				Parts:           enrichedParts,
				Aggregated:      msg.Aggregated,
				SegmentCount:    len(rawSegments),
				RawMessage:      msg.Message,
				ReceivedAt:      endedAt,
				StartedAt:       startedAt,
				EndedAt:         endedAt,
			}

			if err := pipeline.Run(&messageCtx); err != nil {
				var skipErr *inbound.SkipError
				if errors.As(err, &skipErr) {
					clearPendingUserTurnIfMarked(sessionID)
					utils.Infow("message skipped",
						utils.String("request_id", messageCtx.RequestID),
						utils.String("session_id", sessionID),
						utils.Int64("user_id", msg.User_id),
						utils.Int64("group_id", msg.Group_id),
						utils.Int64("message_id", msg.MessageID),
						utils.Bool("aggregated", messageCtx.Aggregated),
						utils.Int("segment_count", messageCtx.SegmentCount),
						utils.String("reason", messageCtx.DropReason),
					)
					metrics.IncCounter(
						"bot_ws_messages_total",
						"Total WebSocket messages by lifecycle result.",
						map[string]string{"result": "skipped"},
					)
					if msg.Message == "exit();" && messageCtx.DropReason == "filter: control command" {
						utils.Info("收到退出命令")
						return
					}
					continue
				}
				clearPendingUserTurnIfMarked(sessionID)
				utils.Errorw("message pipeline failed",
					utils.String("request_id", messageCtx.RequestID),
					utils.String("session_id", sessionID),
					utils.Int64("user_id", msg.User_id),
					utils.Int64("group_id", msg.Group_id),
					utils.Int64("message_id", msg.MessageID),
					utils.Err(err),
				)
				metrics.IncCounter(
					"bot_ws_messages_total",
					"Total WebSocket messages by lifecycle result.",
					map[string]string{"result": "pipeline_error"},
				)
				continue
			}

			state.GetManager().EnsureSession(sessionID, msg.User_id, msg.Group_id, msg.Type)
			state.GetManager().BeginUserTurn(sessionID, startedAt)
			userTurnEnded := false
			endUserTurn := func() {
				if userTurnEnded {
					return
				}
				state.GetManager().EndUserTurn(sessionID, time.Now())
				userTurnEnded = true
			}
			appendTurnGateEvent(ctx, eventStore, "user_turn_started", sessionID, msg.User_id, map[string]any{
				"request_id": messageCtx.RequestID,
				"message_id": msg.MessageID,
			})
			timing := state.GetManager().GetTimingSnapshot(sessionID)
			messageCtx.PreviousUserMessageAt = timing.LastUserMessageAt
			messageCtx.PreviousAssistantMessageAt = timing.LastAssistantMessageAt
			messageCtx.PreviousInteractionAt = timing.LastInteractionAt
			recordIncomingConversationTurn(messageCtx)
			if naturalScheduler != nil && !state.GetManager().GetProactiveSchedule(sessionID).Manual {
				naturalScheduler.RescheduleFrom(sessionID, endedAt)
			}

			utils.Infow("message processing started",
				utils.String("request_id", messageCtx.RequestID),
				utils.String("session_id", sessionID),
				utils.Int64("user_id", msg.User_id),
				utils.Int64("group_id", msg.Group_id),
				utils.Int64("message_id", msg.MessageID),
				utils.Bool("aggregated", messageCtx.Aggregated),
				utils.Int("segment_count", messageCtx.SegmentCount),
				utils.String("message", messageCtx.Message),
				utils.Int("state", int(state.GetManager().GetState(sessionID))),
			)

			result, err := processMessageTurn(ctx, c, messageCtx, processor, agentRuntime, eventStore)
			if err != nil {
				utils.Errorw("message processing failed",
					utils.String("request_id", messageCtx.RequestID),
					utils.String("session_id", sessionID),
					utils.Int64("user_id", msg.User_id),
					utils.Int64("group_id", msg.Group_id),
					utils.Int64("message_id", msg.MessageID),
					utils.Err(err),
				)
				metrics.IncCounter(
					"bot_ws_messages_total",
					"Total WebSocket messages by lifecycle result.",
					map[string]string{"result": "handler_error"},
				)
				endUserTurn()
				appendTurnGateEvent(ctx, eventStore, "user_turn_completed", sessionID, msg.User_id, map[string]any{
					"request_id": messageCtx.RequestID,
					"result":     "handler_error",
				})
				continue
			}

			if cfg.EnableEmotionalMemory && result.Handled && !result.MemoryManaged {
				service.UpdateLongTermMemory(
					sessionID,
					msg.User_id,
					messageCtx.Message,
					result.Reply,
					result.Emotion,
					result.Intention,
				)
			}

			if result.Replied {
				recordedAt := time.Now()
				recordAssistantConversationTurn(sessionID, result.Reply, false, recordedAt)
				if naturalScheduler != nil && !result.ScheduleManaged {
					naturalScheduler.RescheduleFrom(sessionID, recordedAt)
				}
			}
			if result.ReplyMode != "" {
				state.GetManager().UpdateLastReplyMode(sessionID, string(result.ReplyMode))
			}

			utils.Infow("message processing completed",
				utils.String("request_id", messageCtx.RequestID),
				utils.String("session_id", sessionID),
				utils.Int64("user_id", msg.User_id),
				utils.Int64("group_id", msg.Group_id),
				utils.Int64("message_id", msg.MessageID),
				utils.Bool("aggregated", messageCtx.Aggregated),
				utils.Int("segment_count", messageCtx.SegmentCount),
				utils.Bool("handled", result.Handled),
				utils.Bool("replied", result.Replied),
				utils.String("reply_mode", string(result.ReplyMode)),
				utils.String("emotion", result.Emotion),
				utils.String("intention", result.Intention),
				utils.String("reply", result.Reply),
				utils.Int("state", int(state.GetManager().GetState(sessionID))),
			)
			metrics.IncCounter(
				"bot_ws_messages_total",
				"Total WebSocket messages by lifecycle result.",
				map[string]string{"result": "processed"},
			)
			endUserTurn()
			appendTurnGateEvent(ctx, eventStore, "user_turn_completed", sessionID, msg.User_id, map[string]any{
				"request_id": messageCtx.RequestID,
				"result":     "processed",
				"replied":    result.Replied,
			})
		}
	}
}

func processMessageTurn(ctx context.Context, c *websocket.Conn, messageCtx handler.MessageContext,
	processor *handler.MessageProcessor, agentRuntime *agent.RuntimeHandle, eventStore eventlog.Store,
) (*handler.ProcessResult, error) {
	cfg := config.GetConfig()
	runtime := agentRuntime.Get()
	if cfg.EnableReactAgent && runtime != nil {
		turn := agent.NewTurnContext(agent.TurnInput{
			RequestID:                  messageCtx.RequestID,
			SessionID:                  messageCtx.SessionID,
			UserID:                     messageCtx.UserID,
			GroupID:                    messageCtx.GroupID,
			ChatType:                   messageCtx.ChatType,
			Message:                    messageCtx.Message,
			Parts:                      messageCtx.Parts,
			ReferenceTime:              messageCtx.ReceivedAt,
			StartedAt:                  messageCtx.StartedAt,
			EndedAt:                    messageCtx.EndedAt,
			Aggregated:                 messageCtx.Aggregated,
			SegmentCount:               messageCtx.SegmentCount,
			RawSegments:                messageCtx.RawSegments,
			RawSegmentTimes:            messageCtx.RawSegmentTimes,
			PreviousUserMessageAt:      messageCtx.PreviousUserMessageAt,
			PreviousAssistantMessageAt: messageCtx.PreviousAssistantMessageAt,
			PreviousInteractionAt:      messageCtx.PreviousInteractionAt,
			Trigger:                    agent.TriggerMessage,
			Actor:                      agent.ActorUser,
		})

		agentResult, err := runtime.RunTurn(ctx, turn)
		if err == nil {
			reply := strings.TrimSpace(agentResult.FinalReply.Content)
			if agentResult.ShouldSend && reply != "" {
				if sendErr := service.SendMsg(c, messageCtx.UserID, reply); sendErr != nil {
					return nil, sendErr
				}
				agentResult.Events = append(agentResult.Events, eventlog.Event{
					Type:      "reply_sent",
					SessionID: messageCtx.SessionID,
					UserID:    messageCtx.UserID,
					Actor:     "runtime",
					Message:   service.BuildAssistantTranscript(reply),
					CreatedAt: time.Now(),
				})
			}
			if eventStore != nil {
				if appendErr := eventStore.Append(ctx, agentResult.Events...); appendErr != nil {
					utils.Warn("append agent events failed: %v", appendErr)
				}
			}
			return &handler.ProcessResult{
				Handled:         agentResult.Handled,
				Replied:         agentResult.ShouldSend && reply != "",
				MemoryManaged:   true,
				ScheduleManaged: agentResult.ScheduleManaged,
				ReplyMode:       service.ReplyModeFullReply,
				Reply:           service.BuildAssistantTranscript(reply),
			}, nil
		}

		utils.Warn("React agent failed, fallback to legacy handler: %v", err)
	}

	return processor.Process(c, messageCtx)
}

func recordIncomingConversationTurn(messageCtx handler.MessageContext) {
	if userMessage, ok := handler.BuildConversationUserMessage(messageCtx); ok {
		state.GetManager().RecordUserTurn(messageCtx.SessionID, userMessage, messageCtx.EndedAt)
		return
	}

	state.GetManager().RecordUserTurn(messageCtx.SessionID, openai.ChatCompletionMessage{
		Role:    "user",
		Content: messageCtx.Message,
	}, messageCtx.EndedAt)
}

func recordAssistantConversationTurn(sessionID, reply string, proactive bool, recordedAt time.Time) {
	trimmed := strings.TrimSpace(service.BuildAssistantTranscript(reply))
	if trimmed == "" {
		return
	}
	state.GetManager().RecordAssistantTurn(sessionID, trimmed, recordedAt, proactive)
}

func clearPendingUserTurnIfMarked(sessionID string) {
	if state.GetManager().HasPendingOrRunningUserTurn(sessionID) {
		state.GetManager().EndUserTurn(sessionID, time.Now())
	}
}

func appendTurnGateEvent(ctx context.Context, store eventlog.Store, eventType string, sessionID string, userID int64, data map[string]any) {
	if store == nil {
		return
	}
	if err := store.Append(ctx, eventlog.Event{
		Type:      eventType,
		SessionID: sessionID,
		UserID:    userID,
		Actor:     "runtime",
		Data:      data,
		CreatedAt: time.Now(),
	}); err != nil {
		utils.Warn("append turn gate event failed: %v", err)
	}
}

func buildMessageRequestID(messageID int64) string {
	if messageID != 0 {
		return fmt.Sprintf("msg-%d", messageID)
	}
	return utils.NewRequestID("msg")
}
