# Quick Start

这个文档用于把项目尽快跑起来。

## 1. 环境要求

- Go `1.23.6+`
- Node.js `18+`
- 一个可用的 OneBot WebSocket 服务端
- 一个可用的 AI 接口

## 2. 初始化配置

在项目根目录复制环境文件：

```powershell
Copy-Item .env.example .env
```

至少需要确认这些配置：

- `HOSTADD`
- `WsPort`
- `HttpPort`
- `Token`
- `TARGETID`
- `AI_PROFILE`

如果你不使用 `AI_PROFILE` 配置文件方式，也可以直接填写：

- `AI_KEY`
- `AI_BASEURL`
- `AI_MODEL`

## 3. 安装依赖

后端：

```powershell
go mod download
```

前端：

```powershell
Set-Location web
npm install
Set-Location ..
```

## 4. 启动机器人

```powershell
go run ./cmd/bot
```

启动后会同时拉起：

- OneBot WebSocket 机器人主逻辑
- 管理后台 HTTP 服务
- 健康检查和指标接口

默认管理后台地址：

- `http://127.0.0.1:8088`

如果你修改了 `HttpPort`，这里也会跟着变化。

## 5. 开发模式启动前端

如果需要单独调试前端：

```powershell
Set-Location web
npm run dev
```

默认地址：

- `http://localhost:5173`

前端会把 `/api` 代理到后端管理服务。

## 6. 生产构建前端

```powershell
Set-Location web
npm run build
Set-Location ..
```

构建产物会输出到：

- `web/dist`

Go 管理后台会直接托管这批静态文件。

## 7. 核验启动是否正常

浏览器或命令行检查：

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

PowerShell 示例：

```powershell
Invoke-WebRequest http://127.0.0.1:8088/healthz
Invoke-WebRequest http://127.0.0.1:8088/readyz
Invoke-WebRequest http://127.0.0.1:8088/metrics
```

## 8. 建议先关注的配置项

### 连接和身份

- `HOSTADD`
- `WsPort`
- `Token`
- `TARGETID`

### AI

- `AI_PROFILE`
- `AI_KEY`
- `AI_BASEURL`
- `AI_MODEL`
- `AI_PROMPT`

### 行为开关

- `ENABLE_EMOTIONAL_MEMORY`
- `ENABLE_NATURAL_SCHEDULER`
- `ENABLE_ONLY_LONG_CHAT`

### 消息聚合

- `MESSAGE_AGGREGATE_IDLE_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_MESSAGES`

## 9. 常见第一次启动问题

- 后端能启动但不回复：先确认 `TARGETID` 是否正确，并检查消息是否来自私聊
- `/readyz` 返回失败：通常是 `DATA_DIR` 或 `LOG_DIR` 不可写
- 前端空白或接口报错：先确认后端已启动，并检查 `HttpPort`
- AI 不回复：先检查 `AI_PROFILE` 或 `AI_KEY` / `AI_MODEL` 是否配置完整

更详细的排查说明见 [HELP.md](./HELP.md)
