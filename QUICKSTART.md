# Quick Start

这个文档用于把项目尽快跑起来，并说明当前推荐的配置方式。

## 1. 环境要求

- Go `1.23.6+`
- Node.js `18+`
- 一个可用的 OneBot WebSocket 服务端
- 一个 OpenAI-compatible AI 接口

## 2. 初始化环境文件

在项目根目录复制环境文件：

```powershell
Copy-Item .env.example .env
```

至少确认这些连接配置：

- `HOSTADD`
- `WsPort`
- `HttpPort`
- `Token`
- `TARGETID`

`TARGETID` 是机器人主要响应的目标 ID。当前逻辑通常会先用它判断消息是否属于目标会话。

## 3. 初始化 AI profile

推荐使用 AI profile 文件管理模型配置：

```powershell
Copy-Item config\ai_profiles.example.json config\ai_profiles.json
```

然后编辑 `config/ai_profiles.json`：

```json
{
  "active": "default",
  "profiles": {
    "default": {
      "aiBaseUrl": "https://api.openai.com/v1",
      "aiModel": "gpt-4o-mini",
      "aiKey": "your_api_key",
      "aiTemperature": 1,
      "aiMaxTokens": 2000,
      "aiTimeout": 30,
      "aiRetryCount": 3,
      "aiRateLimit": 20,
      "aiTopP": 0.9
    }
  }
}
```

`.env` 中只需要保留选择器：

```env
AI_PROFILE=default
AI_CONFIG_FILE=./config/ai_profiles.json
```

`AI_KEY`、`AI_BASEURL`、`AI_MODEL` 等环境变量仍然可用，但现在主要作为 profile 文件缺失或首次生成 profile 时的兜底值。日常修改模型参数时，优先改 `config/ai_profiles.json` 或通过管理后台修改。

`config/ai_profiles.json` 会被 `.gitignore` 忽略，不要提交真实 key。

## 4. 确认 Character Card

角色配置来自 `config/character/*.json`。默认配置是：

```env
CHARACTER=default
```

这会加载：

```text
config/character/default.json
```

该文件是 Character Card 的来源，负责角色身份、语气、边界和示例回复。它不是 ReAct 工具规则，也不应该承载工具调用策略。

如果你新增角色，例如 `config/character/demo.json`，则把 `.env` 改成：

```env
CHARACTER=demo
```

角色文件缺失会导致启动失败。

## 5. 安装依赖

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

## 6. 启动机器人

```powershell
go run ./cmd/bot
```

启动后会同时拉起：

- OneBot WebSocket 机器人主逻辑
- ReAct/legacy 回复运行时
- 自然调度器
- 管理后台 HTTP 服务
- 健康检查和指标接口

默认管理后台地址：

- `http://127.0.0.1:8088`

如果修改了 `HttpPort`，这里也会跟着变化。

## 7. 开启 ReAct 工具模式

当前可以通过 `.env` 控制是否启用 ReAct Agent：

```env
ENABLE_REACT_AGENT=false
REACT_MAX_STEPS=4
REACT_TOOL_TIMEOUT_MS=3000
REACT_ALLOW_WRITE_TOOLS=false
REACT_TRACE_MODE=basic
REACT_TOTAL_TIMEOUT_MS=30000
```

建议第一次启动先保持 `ENABLE_REACT_AGENT=false`，确认 OneBot、AI 和管理后台都正常后，再改为：

```env
ENABLE_REACT_AGENT=true
```

如果希望 ReAct 工具可以写入长期记忆、好感等状态，还需要开启：

```env
REACT_ALLOW_WRITE_TOOLS=true
```

只读工具不需要这个开关。写工具会产生业务状态变化，建议先在测试会话中验证。

## 8. 开发模式启动前端

如果需要单独调试前端：

```powershell
Set-Location web
npm run dev
```

默认地址：

- `http://localhost:5173`

前端会把 `/api` 代理到后端管理服务。

## 9. 生产构建前端

```powershell
Set-Location web
npm run build
Set-Location ..
```

构建产物会输出到：

- `web/dist`

Go 管理后台会直接托管这批静态文件。

## 10. 核验启动是否正常

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

## 11. 建议先关注的配置项

### 连接和身份

- `HOSTADD`
- `WsPort`
- `Token`
- `TARGETID`

### AI

- `AI_PROFILE`
- `AI_CONFIG_FILE`
- `config/ai_profiles.json`

### 角色

- `CHARACTER`
- `config/character/default.json`
- `CHARACTER_IDENTITY_MODE`

### ReAct

- `ENABLE_REACT_AGENT`
- `REACT_MAX_STEPS`
- `REACT_ALLOW_WRITE_TOOLS`
- `REACT_TRACE_MODE`

### 行为开关

- `ENABLE_EMOTIONAL_MEMORY`
- `ENABLE_NATURAL_SCHEDULER`
- `ENABLE_ONLY_LONG_CHAT`

### 消息聚合

- `MESSAGE_AGGREGATE_IDLE_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_MESSAGES`

## 12. 常见第一次启动问题

- 后端能启动但不回复：先确认 `TARGETID` 是否正确，并检查消息来源是否符合当前过滤规则。
- 启动时报 character config not found：确认 `CHARACTER` 对应的 `config/character/<name>.json` 存在。
- AI 不回复：先确认 `config/ai_profiles.json` 的 active profile、`aiKey`、`aiBaseUrl` 和 `aiModel`。
- `/readyz` 返回失败：通常是 `DATA_DIR` 或 `LOG_DIR` 不可写。
- 前端空白或接口报错：先确认后端已启动，并检查 `HttpPort`。
- ReAct 工具没有写入状态：确认 `ENABLE_REACT_AGENT=true`，并在需要副作用时设置 `REACT_ALLOW_WRITE_TOOLS=true`。

更详细的排查说明见 [HELP.md](./HELP.md)。
