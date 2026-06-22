# ReEscape Protocol

一个基于 OneBot WebSocket 的聊天机器人系统，包含会话状态隔离、消息聚合、AI 分类回复、长期记忆、自然定时和 Web 管理后台。

## 系统概览

- `cmd/bot` 负责启动机器人主进程
- `internal/inbound` 负责消息入站处理
- `internal/handler` 负责回复决策
- `internal/state` 负责按用户/会话维护状态和对话
- `internal/memory` 负责情绪、画像和事实记忆
- `internal/admin` 负责健康检查、指标、配置和日志管理
- `web/` 提供管理后台前端

## 主要能力

- 预设回复和长对话
- 情绪、意图、结束意图的结构化分类
- 消息聚合：把连续碎片消息合并成一次上下文
- 按用户/会话隔离状态和对话历史
- 情绪记忆、长期偏好和事实记忆
- 自然定时发送
- 结构化日志、健康检查、就绪检查、Prometheus 指标
- 角色配置和在线管理后台

## 配置分类

- 连接：`HOSTADD`、`WsPort`、`HttpPort`、`Token`、`TARGETID`
- AI：`AI_PROFILE`、`AI_KEY`、`AI_BASEURL`、`AI_MODEL`、`AI_PROMPT`
- 行为：`ENABLE_EMOTIONAL_MEMORY`、`ENABLE_NATURAL_SCHEDULER`、`ENABLE_ONLY_LONG_CHAT`
- 聚合：`MESSAGE_AGGREGATE_IDLE_WINDOW_MS`、`MESSAGE_AGGREGATE_MAX_WINDOW_MS`、`MESSAGE_AGGREGATE_MAX_MESSAGES`
- 运行：`DATA_DIR`、`LOG_DIR`、`LOG_LEVEL`、`LOG_FORMAT`

## 入站链路

`raw message -> aggregate -> dedupe -> filter -> normalize -> dispatch`

## 目录结构

- `cmd/bot`：程序入口
- `internal/config`：环境变量和运行时配置
- `internal/inbound`：入站处理链路
- `internal/handler`：消息回复逻辑
- `internal/memory`：情绪/画像/事实记忆
- `internal/state`：会话状态与对话历史
- `internal/admin`：管理后台 HTTP 服务
- `web`：管理后台前端

## 快速开始

见 [QUICKSTART.md](./QUICKSTART.md)

## 帮助

见 [HELP.md](./HELP.md)

## Web 管理后台

前端说明见 [web/README.md](./web/README.md)
