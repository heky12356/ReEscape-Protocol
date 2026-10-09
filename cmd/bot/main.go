package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"project-yume/internal/app"
	"project-yume/internal/config"
	"project-yume/internal/utils"
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
	if err := utils.ConfigureAIRawLogger(cfg.EnableAIRawLog, cfg.LogDir); err != nil {
		fmt.Fprintf(os.Stderr, "configure ai raw logger failed: %v\n", err)
	}

	utils.Info("启动 ReEscape Protocol 聊天机器人...")
	utils.Info("配置加载完成 - 目标用户: %d", cfg.TargetId)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	components, err := app.Bootstrap(ctx)
	if err != nil {
		utils.Error("应用启动失败: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := components.Stop(); err != nil {
			utils.Error("最终状态刷盘失败: %v", err)
		}
	}()

	go app.RunConnectionSupervisor(ctx, components)

	utils.Info("所有服务已启动，机器人开始工作...")

	select {
	case <-ctx.Done():
		utils.Info("程序正常退出")
	case <-interrupt:
		utils.Info("接收到中断信号，正在关闭...")
		cancel()
		shutdown := time.Duration(cfg.ShutdownTimeoutMs) * time.Millisecond
		if shutdown <= 0 {
			shutdown = 10 * time.Second
		}
		time.Sleep(shutdown)
	}
}
