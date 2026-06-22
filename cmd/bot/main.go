package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"project-yume/internal/admin"
	"project-yume/internal/config"
	"project-yume/internal/connect"
	"project-yume/internal/handler"
	"project-yume/internal/inbound"
	"project-yume/internal/memory"
	"project-yume/internal/metrics"
	"project-yume/internal/model"
	"project-yume/internal/scheduler"
	"project-yume/internal/service"
	"project-yume/internal/state"
	"project-yume/internal/storage"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
	"github.com/sashabaranov/go-openai"
)

func main() {
	cfg := config.GetConfig()

	if err := utils.ConfigureDefaultLogger(
		utils.ParseLogLevel(cfg.LogLevel),
		cfg.LogToFile,
		cfg.LogEnableColor,
		cfg.LogDir,
		cfg.LogFormat,
	); err != nil {
		fmt.Fprintf(os.Stderr, "configure logger failed: %v\n", err)
	}

	utils.Info("启动 ReEscape Protocol 聊天机器人...")
	utils.Info("配置加载完成 - 目标用户: %d", cfg.TargetId)

	// 建立连接
	c, err := connect.Init(cfg.Hostadd + ":" + cfg.WsPort)
	if err != nil {
		utils.Error("连接失败: %v", err)
		os.Exit(1)
	}
	defer connect.Close(c)
	utils.Info("WebSocket连接成功: %s", cfg.Hostadd)

	// 初始化组件
	// 初始化消息处理器
	messageProcessor := handler.NewMessageProcessor()
	messagePipeline := inbound.NewPipeline(
		inbound.NewDedupeStage(5*time.Minute),
		inbound.NewFilterStage(),
		inbound.NewNormalizeStage(),
	)

	// 定义自然定时器
	var naturalScheduler *scheduler.NaturalScheduler
	// 通过config查看是否启用自然定时器
	if cfg.EnableNaturalScheduler {
		naturalScheduler = scheduler.NewNaturalScheduler()
		utils.Info("自然定时器已启用")
	}

	if cfg.EnableEmotionalMemory {
		utils.Info("情感记忆系统已启用")
	}

	if cfg.EnableOnlyLongChat {
		utils.Info("仅长聊天模式已启用")
	}
	utils.Info("消息聚合已启用: idle=%dms max_window=%dms max_messages=%d",
		cfg.MessageAggregateIdleWindowMs,
		cfg.MessageAggregateMaxWindowMs,
		cfg.MessageAggregateMaxMessages,
	)

	// 监听中断信号
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// 定义上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshotStore := storage.NewFileSnapshotStore(cfg.DataDir)
	flushWorker := storage.NewFlushWorker(2 * time.Second)
	if err := memory.GetManager().ConfigurePersistence(snapshotStore, flushWorker); err != nil {
		utils.Error("配置情感记忆持久化失败: %v", err)
		os.Exit(1)
	}
	if err := memory.GetProfileManager().ConfigurePersistence(snapshotStore, flushWorker); err != nil {
		utils.Error("配置用户画像持久化失败: %v", err)
		os.Exit(1)
	}
	if err := memory.GetFactManager().ConfigurePersistence(snapshotStore, flushWorker); err != nil {
		utils.Error("配置事实记忆持久化失败: %v", err)
		os.Exit(1)
	}
	if err := state.GetManager().ConfigurePersistence(snapshotStore, flushWorker); err != nil {
		utils.Error("配置会话持久化失败: %v", err)
		os.Exit(1)
	}
	flushWorker.Register(memory.FlushTaskName, memory.GetManager().Flush)
	flushWorker.Register(memory.ProfileFlushTaskName, memory.GetProfileManager().Flush)
	flushWorker.Register(memory.FactFlushTaskName, memory.GetFactManager().Flush)
	flushWorker.Register(state.FlushTaskName, state.GetManager().Flush)
	go flushWorker.Run(ctx)
	defer flushWorker.Stop()

	// 启动管理后台 HTTP 服务
	go admin.Start(ctx)

	// 定义消息通道
	rawMsgChan := make(chan model.Msg, 100)
	aggregatedMsgChan := make(chan model.Msg, 100)

	// 启动消息接收协程
	go startMessageReceiver(c, rawMsgChan, ctx)

	// 启动消息聚合协程
	go inbound.NewMessageAggregator().Run(ctx, rawMsgChan, aggregatedMsgChan)

	// 启动消息处理协程
	go startMessageProcessor(c, aggregatedMsgChan, messagePipeline, messageProcessor, naturalScheduler, ctx)

	// 启动定时任务协程
	if naturalScheduler != nil {
		go startScheduler(c, naturalScheduler, ctx, state.PrivateSessionID(cfg.TargetId), cfg.TargetId)
	}

	// 启动状态监控协程
	go startStatusMonitor(ctx, state.PrivateSessionID(cfg.TargetId))

	utils.Info("所有服务已启动，机器人开始工作...")

	// 主循环，处理中断信号
	for {
		select {
		case <-ctx.Done():
			utils.Info("程序正常退出")
			return
		case <-interrupt:
			utils.Info("接收到中断信号，正在关闭...")

			// 优雅关闭
			err := connect.WriteMessage(c, websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				utils.Error("发送关闭消息失败: %v", err)
			}

			// 等待协程结束或超时
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				utils.Info("等待超时，强制退出")
			}

			cancel()
			return
		}
	}
}

// startMessageReceiver 启动消息接收协程
func startMessageReceiver(c *websocket.Conn, msgChan chan model.Msg, ctx context.Context) {
	defer close(msgChan)

	for {
		select {
		case <-ctx.Done():
			utils.Info("消息接收器已停止")
			return
		default:
			_, message, err := c.ReadMessage()
			if err != nil {
				utils.Error("读取消息失败: %v", err)
				time.Sleep(time.Second) // 避免快速重试
				continue
			}

			utils.Info("接收到消息: %s", message)

			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(message, &envelope); err == nil {
				if _, hasPostType := envelope["post_type"]; !hasPostType {
					if connect.DispatchAPIResponse(message) {
						continue
					}
				}
			}

			metrics.IncCounter(
				"bot_ws_messages_total",
				"Total WebSocket messages by lifecycle result.",
				map[string]string{"result": "received"},
			)

			var msg model.Response
			err = json.Unmarshal(message, &msg)
			if err != nil {
				utils.Error("消息反序列化失败: %v", err)
				metrics.IncCounter(
					"bot_ws_messages_total",
					"Total WebSocket messages by lifecycle result.",
					map[string]string{"result": "decode_error"},
				)
				continue
			}

			// 转换为内部消息格式
			internalMsg := model.Msg{
				Message:   msg.Raw_message,
				Parts:     buildIncomingMessageParts(msg),
				User_id:   msg.User_id,
				Group_id:  msg.Group_id,
				MessageID: msg.Message_id,
				Time:      msg.Time,
				Type: func() int {
					if msg.Message_type == "group" {
						return 0
					}
					return 1
				}(),
			}

			// 非阻塞发送到处理通道
			select {
			case msgChan <- internalMsg:
			case <-time.After(100 * time.Millisecond):
				utils.Warn("消息通道满，丢弃消息")
				metrics.IncCounter(
					"bot_ws_messages_total",
					"Total WebSocket messages by lifecycle result.",
					map[string]string{"result": "channel_dropped"},
				)
			}
		}
	}
}

// startMessageProcessor 启动消息处理协程
func startMessageProcessor(c *websocket.Conn, msgChan chan model.Msg,
	pipeline *inbound.Pipeline, processor *handler.MessageProcessor, naturalScheduler *scheduler.NaturalScheduler, ctx context.Context,
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
			messageCtx := handler.MessageContext{
				RequestID:    buildMessageRequestID(msg.MessageID),
				SessionID:    sessionID,
				UserID:       msg.User_id,
				GroupID:      msg.Group_id,
				ChatType:     msg.Type,
				MessageID:    msg.MessageID,
				MessageIDs:   messageIDs,
				RawSegments:  rawSegments,
				Parts:        enrichedParts,
				Aggregated:   msg.Aggregated,
				SegmentCount: len(rawSegments),
				RawMessage:   msg.Message,
				ReceivedAt:   endedAt,
				StartedAt:    startedAt,
				EndedAt:      endedAt,
			}

			if err := pipeline.Run(&messageCtx); err != nil {
				var skipErr *inbound.SkipError
				if errors.As(err, &skipErr) {
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
			recordIncomingConversationTurn(messageCtx)
			if naturalScheduler != nil {
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

			// 使用新的消息处理器获取详细结果
			result, err := processor.Process(c, messageCtx)
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
				continue
			}

			// 记录到情感记忆（如果启用）
			if cfg.EnableEmotionalMemory && result.Handled {
				if result.Emotion != "" && result.Intention != "" {
					utils.Info("记录情感记忆 - 情感: %s, 意图: %s", result.Emotion, result.Intention)
				} else {
					utils.Warn("情绪交互记录将跳过，但仍尝试提取长期偏好/事实: emotion=%q intention=%q", result.Emotion, result.Intention)
				}
				service.UpdateLongTermMemory(
					sessionID,
					msg.User_id,
					messageCtx.Message,
					result.Reply,
					result.Emotion,
					result.Intention,
				)
			}

			// 更新状态
			if result.Replied {
				recordedAt := time.Now()
				recordAssistantConversationTurn(sessionID, result.Reply, false, recordedAt)
				if naturalScheduler != nil {
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
		}
	}
}

// startScheduler 启动定时任务协程
func startScheduler(c *websocket.Conn, scheduler *scheduler.NaturalScheduler, ctx context.Context, sessionID string, targetUserID int64) {
	// 初始延迟
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
			err := scheduler.SendScheduledMessage(c, sessionID, targetUserID)
			if err != nil {
				utils.Error("定时消息发送失败: %v", err)
			} else {
				utils.Info("定时器触发成功")
			}
		}
	}
}

// startStatusMonitor 启动状态监控协程
func startStatusMonitor(ctx context.Context, sessionID string) {
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

func buildMessageRequestID(messageID int64) string {
	if messageID != 0 {
		return fmt.Sprintf("msg-%d", messageID)
	}
	return utils.NewRequestID("msg")
}

func buildIncomingMessageParts(resp model.Response) []model.MessagePart {
	parts := make([]model.MessagePart, 0, len(resp.Message))

	for _, segment := range resp.Message {
		switch segment.Type {
		case "text":
			text := strings.TrimSpace(segment.Data["text"])
			if text == "" {
				continue
			}
			parts = append(parts, model.MessagePart{
				Type: "text",
				Text: text,
			})
		case "image":
			parts = append(parts, model.MessagePart{
				Type: "image",
				URL:  strings.TrimSpace(segment.Data["url"]),
				File: strings.TrimSpace(segment.Data["file"]),
			})
		}
	}

	if len(parts) > 0 {
		return parts
	}

	raw := strings.TrimSpace(resp.Raw_message)
	if raw == "" {
		return nil
	}
	if utils.IsCQImage(raw) {
		return []model.MessagePart{{
			Type: "image",
			URL:  strings.TrimSpace(utils.ExtractImageURL(raw)),
			File: strings.TrimSpace(utils.ExtractImageFile(raw)),
		}}
	}
	return []model.MessagePart{{
		Type: "text",
		Text: raw,
	}}
}
