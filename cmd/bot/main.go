package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"project-yume/internal/app"
	"project-yume/internal/config"
	"project-yume/internal/connect"
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
	if err := utils.ConfigureAIRawLogger(cfg.EnableAIRawLog, cfg.LogDir); err != nil {
		fmt.Fprintf(os.Stderr, "configure ai raw logger failed: %v\n", err)
	}

	utils.Info("启动 ReEscape Protocol 聊天机器人...")
	utils.Info("配置加载完成 - 目标用户: %d", cfg.TargetId)

	c, err := connect.Init(cfg.Hostadd + ":" + cfg.WsPort)
	if err != nil {
		utils.Error("连接失败: %v", err)
		os.Exit(1)
	}
	defer connect.Close(c)
	utils.Info("WebSocket连接成功: %s", cfg.Hostadd)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	components, err := app.Bootstrap(ctx)
	if err != nil {
		utils.Error("应用启动失败: %v", err)
		os.Exit(1)
	}
	defer components.Stop()

	app.StartWorkers(ctx, c, components)

	utils.Info("所有服务已启动，机器人开始工作...")

	for {
		select {
		case <-ctx.Done():
			utils.Info("程序正常退出")
			return
		case <-interrupt:
			utils.Info("接收到中断信号，正在关闭...")

			err := connect.WriteMessage(c, websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				utils.Error("发送关闭消息失败: %v", err)
			}

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
