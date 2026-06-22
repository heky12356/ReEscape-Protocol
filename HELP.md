# Help

这个文档用于说明当前项目的体系、入口和排查方式。

## 1. 当前体系

### 机器人主链路

- WebSocket 接收 OneBot 上报消息
- 消息先进入聚合层
- 再进入 `dedupe -> filter -> normalize`
- 最后交给消息处理器决定回复策略

### 回复策略

- 预设回复
- 情绪分析回复
- 长对话继续/结束

### 记忆与状态

- 会话状态按 `userID/sessionID` 隔离
- 对话历史按会话保存
- 情绪记忆、长期偏好、事实记忆会异步刷盘

### 管理后台

- 配置查看和热更新
- 角色配置管理
- AI Profile 查看
- 日志查看和流式输出
- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## 2. 目录里最常用的文件

- `README.md`：项目总览
- `QUICKSTART.md`：快速启动
- `HELP.md`：排查和说明
- `.env.example`：环境变量样例
- `config/character/`：角色配置
- `web/dist/`：前端构建产物

## 3. 配置说明

### 必填项

- `HOSTADD`
- `WsPort`
- `HttpPort`
- `Token`
- `TARGETID`

### 推荐检查项

- `AI_PROFILE`
- `AI_KEY`
- `AI_BASEURL`
- `AI_MODEL`
- `ENABLE_EMOTIONAL_MEMORY`
- `ENABLE_NATURAL_SCHEDULER`
- `ENABLE_ONLY_LONG_CHAT`
- `MESSAGE_AGGREGATE_IDLE_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_MESSAGES`

## 4. 健康检查

### `GET /healthz`

看进程是否存活。

### `GET /readyz`

看当前是否适合接流量，通常会检查配置和目录可写性。

### `GET /metrics`

给 Prometheus 抓取指标。

## 5. 常见问题

### 启动后没有回复

先检查：

- `TARGETID` 是否匹配
- 是否是私聊消息
- `ENABLE_ONLY_LONG_CHAT` 是否把流程切到长对话
- 日志里是否有 `pipeline_error` 或 `handler_error`

### 机器人回复太频繁

调整消息聚合参数：

- `MESSAGE_AGGREGATE_IDLE_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_MESSAGES`

### 记忆没有落盘

先检查：

- `DATA_DIR` 是否可写
- `ENABLE_EMOTIONAL_MEMORY` 是否开启
- 管理后台日志里是否有 flush 失败

### 前端打不开

确认：

- 后端已经启动
- `HttpPort` 配置正确
- `web/dist` 已生成或正在使用开发模式

## 6. 建议的检查顺序

1. 看 `healthz`
2. 看 `readyz`
3. 看 `metrics`
4. 看后端日志
5. 看管理后台配置页和日志页
