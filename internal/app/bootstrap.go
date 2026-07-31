package app

import (
	"context"
	"fmt"
	"time"

	"project-yume/internal/admin"
	"project-yume/internal/agent"
	"project-yume/internal/config"
	"project-yume/internal/domain/affection"
	"project-yume/internal/eventlog"
	"project-yume/internal/handler"
	"project-yume/internal/inbound"
	"project-yume/internal/memory"
	"project-yume/internal/model"
	"project-yume/internal/scheduler"
	"project-yume/internal/state"
	"project-yume/internal/storage"
	"project-yume/internal/tools"
	"project-yume/internal/tools/catalog"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

type Components struct {
	MessageProcessor *handler.MessageProcessor
	MessagePipeline  *inbound.Pipeline
	NaturalScheduler *scheduler.NaturalScheduler
	AgentRuntime     *agent.Runtime
	EventStore       eventlog.Store
	FlushWorker      *storage.FlushWorker
}

func Bootstrap(ctx context.Context) (*Components, error) {
	cfg := config.GetConfig()

	components := &Components{
		MessageProcessor: handler.NewMessageProcessor(),
		MessagePipeline: inbound.NewPipeline(
			inbound.NewDedupeStage(5*time.Minute),
			inbound.NewFilterStage(),
			inbound.NewNormalizeStage(),
		),
		AgentRuntime: NewAgentRuntime(),
		FlushWorker:  storage.NewFlushWorker(2 * time.Second),
	}

	if cfg.EnableNaturalScheduler {
		components.NaturalScheduler = scheduler.NewNaturalScheduler()
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

	snapshotStore := storage.NewFileSnapshotStore(cfg.DataDir)
	if err := memory.GetManager().ConfigurePersistence(snapshotStore, components.FlushWorker); err != nil {
		return nil, fmt.Errorf("configure emotional memory persistence: %w", err)
	}
	if err := memory.GetProfileManager().ConfigurePersistence(snapshotStore, components.FlushWorker); err != nil {
		return nil, fmt.Errorf("configure user profile persistence: %w", err)
	}
	if err := memory.GetFactManager().ConfigurePersistence(snapshotStore, components.FlushWorker); err != nil {
		return nil, fmt.Errorf("configure fact memory persistence: %w", err)
	}
	if err := state.GetManager().ConfigurePersistence(snapshotStore, components.FlushWorker); err != nil {
		return nil, fmt.Errorf("configure session persistence: %w", err)
	}
	if err := affection.GetManager().ConfigurePersistence(snapshotStore, components.FlushWorker); err != nil {
		return nil, fmt.Errorf("configure affection persistence: %w", err)
	}

	eventStore, err := eventlog.NewFileStore(snapshotStore, components.FlushWorker)
	if err != nil {
		return nil, fmt.Errorf("configure event log persistence: %w", err)
	}
	components.EventStore = eventStore
	eventlog.SetDefault(eventStore)

	components.FlushWorker.Register(memory.FlushTaskName, memory.GetManager().Flush)
	components.FlushWorker.Register(memory.ProfileFlushTaskName, memory.GetProfileManager().Flush)
	components.FlushWorker.Register(memory.FactFlushTaskName, memory.GetFactManager().Flush)
	components.FlushWorker.Register(state.FlushTaskName, state.GetManager().Flush)
	components.FlushWorker.Register(affection.FlushTaskName, affection.GetManager().Flush)
	components.FlushWorker.Register(eventlog.FlushTaskName, eventStore.Flush)
	go components.FlushWorker.Run(ctx)

	go admin.Start(ctx)

	return components, nil
}

func (c *Components) Stop() {
	if c == nil || c.FlushWorker == nil {
		return
	}
	c.FlushWorker.Stop()
}

func StartWorkers(ctx context.Context, conn *websocket.Conn, components *Components) {
	if components == nil {
		return
	}

	rawMsgChan := make(chan model.Msg, 100)
	aggregatedMsgChan := make(chan model.Msg, 100)

	go startMessageReceiver(conn, rawMsgChan, ctx)
	go inbound.NewMessageAggregator().Run(ctx, rawMsgChan, aggregatedMsgChan)
	go startMessageProcessor(
		conn,
		aggregatedMsgChan,
		components.MessagePipeline,
		components.MessageProcessor,
		components.AgentRuntime,
		components.EventStore,
		components.NaturalScheduler,
		ctx,
	)

	cfg := config.GetConfig()
	sessionID := state.PrivateSessionID(cfg.TargetId)
	if components.NaturalScheduler != nil {
		go startScheduler(conn, components.NaturalScheduler, components.AgentRuntime, components.EventStore, ctx, sessionID, cfg.TargetId)
	}
	go StartStatusMonitor(ctx, sessionID)
}

func NewAgentRuntime() *agent.Runtime {
	registry := catalog.NewRegistry()

	cfg := config.GetConfig()
	toolTimeout := time.Duration(cfg.ReactToolTimeoutMs) * time.Millisecond
	executor := tools.NewExecutor(
		registry,
		tools.Policy{AllowWriteTools: cfg.ReactAllowWriteTools},
		toolTimeout,
	)
	return agent.NewRuntime(registry, executor, agent.BudgetFromConfig())
}
