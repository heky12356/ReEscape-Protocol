package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
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
	"project-yume/internal/state"
	"project-yume/internal/storage"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
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
	if err := state.GetManager().ConfigurePersistence(snapshotStore, flushWorker); err != nil {
		utils.Error("配置会话持久化失败: %v", err)
		os.Exit(1)
	}
	flushWorker.Register(memory.FlushTaskName, memory.GetManager().Flush)
	flushWorker.Register(state.FlushTaskName, state.GetManager().Flush)
	go flushWorker.Run(ctx)
	defer flushWorker.Stop()

	// 启动管理后台 HTTP 服务
	go admin.Start(ctx)

	// 定义消息通道
	msgChan := make(chan model.Msg, 100) // 增加缓冲区

	// 启动消息接收协程
	go startMessageReceiver(c, msgChan, ctx)

	// 启动消息处理协程
	go startMessageProcessor(c, msgChan, messagePipeline, messageProcessor, ctx)

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
	pipeline *inbound.Pipeline, processor *handler.MessageProcessor, ctx context.Context,
) {
	cfg := config.GetConfig()
	memoryManager := memory.GetManager()

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
			messageCtx := handler.MessageContext{
				RequestID:  buildMessageRequestID(msg.MessageID),
				SessionID:  sessionID,
				UserID:     msg.User_id,
				GroupID:    msg.Group_id,
				ChatType:   msg.Type,
				MessageID:  msg.MessageID,
				RawMessage: msg.Message,
				ReceivedAt: time.Unix(msg.Time, 0),
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

			utils.Infow("message processing started",
				utils.String("request_id", messageCtx.RequestID),
				utils.String("session_id", sessionID),
				utils.Int64("user_id", msg.User_id),
				utils.Int64("group_id", msg.Group_id),
				utils.Int64("message_id", msg.MessageID),
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
					memoryManager.RecordInteraction(
						msg.User_id,
						msg.Message,
						result.Reply,
						result.Emotion,
						result.Intention,
					)
				} else {
					utils.Warn("跳过情感记忆写入，emotion/intention 无效: emotion=%q intention=%q", result.Emotion, result.Intention)
				}
			}

			// 更新状态
			state.GetManager().UpdateLastReply(sessionID)

			utils.Infow("message processing completed",
				utils.String("request_id", messageCtx.RequestID),
				utils.String("session_id", sessionID),
				utils.Int64("user_id", msg.User_id),
				utils.Int64("group_id", msg.Group_id),
				utils.Int64("message_id", msg.MessageID),
				utils.Bool("handled", result.Handled),
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

	for {
		select {
		case <-ctx.Done():
			utils.Info("定时器已停止")
			return
		default:
			interval := scheduler.GetNextInterval()
			utils.Info("下次发送间隔: %v", interval)

			timer := time.NewTimer(interval)
			// timer := time.NewTimer(1 * time.Minute)

			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
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
			utils.Info("当前状态(%s): %v, 上次回复: %v",
				sessionID, sm.GetState(sessionID), sm.GetTimeSinceLastReply(sessionID))
		}
	}
}

func buildMessageRequestID(messageID int64) string {
	if messageID != 0 {
		return fmt.Sprintf("msg-%d", messageID)
	}
	return utils.NewRequestID("msg")
}
