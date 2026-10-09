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

生产环境还应设置管理后台密钥。`/api/admin/*` 默认要求 `ADMIN_API_KEY`，管理服务默认只监听 `127.0.0.1`：

```env
ADMIN_LISTEN_HOST=127.0.0.1
ADMIN_API_KEY=请替换为随机长密钥
ADMIN_CORS_ORIGINS=http://127.0.0.1:8088,http://localhost:5173
```

OneBot 连接断开后会自动退避重连，可通过 `ONEBOT_RECONNECT_INITIAL_MS`、`ONEBOT_RECONNECT_MAX_MS`、`ONEBOT_HEARTBEAT_INTERVAL_MS` 和 `ONEBOT_READ_TIMEOUT_MS` 调整。

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
- ReAct 回复运行时
- 自然调度器
- 管理后台 HTTP 服务
- 健康检查和指标接口

默认管理后台地址：

- `http://127.0.0.1:8088`

如果修改了 `HttpPort`，这里也会跟着变化。

## 7. 开启 ReAct 工具模式

当前可以通过 `.env` 控制是否启用 ReAct Agent：

```env
ENABLE_REACT_AGENT=true
REACT_MAX_STEPS=4
REACT_TOOL_TIMEOUT_MS=3000
REACT_ALLOW_WRITE_TOOLS=false
REACT_TRACE_MODE=basic
REACT_TOTAL_TIMEOUT_MS=30000
```

ReAct Runtime 是唯一消息处理链路，启动前应保持：

```env
ENABLE_REACT_AGENT=true
```

如果希望 ReAct 工具可以写入长期记忆、好感、主动触达计划等状态，还需要开启：

```env
REACT_ALLOW_WRITE_TOOLS=true
```

只读工具不需要这个开关。关闭时，`remember_fact`、`update_profile`、`update_affection`、`update_proactive_schedule` 这类写工具不会暴露给模型，也不会出现在 ReAct system prompt 的可用工具列表里。

开启后，模型会根据 Memory Tool Rules 自主判断是否写入记忆。用户明确说“记住”“以后”“我喜欢”“我叫”“不要再”这类表达时，更容易触发 `remember_fact` 或 `update_profile`。写工具会产生业务状态变化，建议先在测试会话中验证。

如需允许 agent 根据用户约定设置下一次主动触达，例如“晚上九点再来找我”“半小时后提醒我一下”或“今天先别主动找我了”，请开启 `REACT_ALLOW_WRITE_TOOLS=true`。查询当前计划的 `get_proactive_schedule` 是只读工具，不需要开启写工具。

## 8. 可选：配置 ReAct Skills

Skills 是 ReAct 的只读策略层，用来给特定场景提供回复流程，例如安慰、冲突修复、主动触达约定或技术解释。默认已启用，并从项目内置目录加载：

```env
ENABLE_SKILLS=true
SKILL_DIRS=./config/skills
SKILL_AUTO_HINT_LIMIT=3
SKILL_RESOURCE_MAX_BYTES=65536
SKILL_ALLOW_SCRIPTS=false
SKILL_LOAD_SYSTEM=false
```

标准 skill 目录格式如下：

```text
config/skills/
  comfort/
    SKILL.md
```

`SKILL.md` 至少需要 `name` 和 `description`：

```markdown
---
name: comfort
description: Use when the user expresses sadness, stress, fatigue, disappointment, or emotional overwhelm.
---

# Comfort

先承接用户情绪，不急着解决问题。
```

运行时只会自动注入匹配到的 skill hints，不会把所有 skill 正文塞进 prompt。模型需要完整策略时会调用只读工具 `read_skill`；需要 `references/`、`assets/` 或 `scripts/` 中的文本资源时会调用 `read_skill_resource`。第一阶段不会执行 `scripts/`。

`SKILL_DIRS` 支持逗号分隔多个目录。新增或删除 skill 后，建议重启机器人；通过管理后台保存运行时配置时也会触发 skill 重新加载。

## 9. 可选：开启 Web Tools

`web_search` 和 `web_fetch` 是 ReAct 只读工具，默认关闭。需要稳定 agent 搜索时推荐 Tavily：

```env
ENABLE_WEB_TOOLS=true
WEB_SEARCH_PROVIDER=tavily
WEB_SEARCH_API_KEY=tvly-xxx
WEB_SEARCH_MAX_RESULTS=5
WEB_TOOL_TIMEOUT_MS=8000
WEB_FETCH_MAX_BYTES=1048576
WEB_FETCH_MAX_CHARS=6000
WEB_FETCH_USER_AGENT=ReEscapeProtocolBot/1.0
```

也可以继续使用本地 SearXNG 作为免费自建方案：

```env
ENABLE_WEB_TOOLS=true
WEB_SEARCH_PROVIDER=searxng
WEB_SEARCH_ENDPOINT=http://127.0.0.1:8888/search
```

SearXNG 需要启用 JSON search API，并把 endpoint 配到 `/search`。公开 SearXNG 实例可能返回 HTML bot challenge，不建议依赖。工具注册类配置修改后建议重启机器人。

## 10. 开发模式启动前端

如果需要单独调试前端：

```powershell
Set-Location web
npm run dev
```

默认地址：

- `http://localhost:5173`

前端会把 `/api` 代理到后端管理服务。

## 11. 生产构建前端

```powershell
Set-Location web
npm run build
Set-Location ..
```

构建产物会输出到：

- `web/dist`

Go 管理后台会直接托管这批静态文件。

## 12. 核验启动是否正常

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

## 13. 建议先关注的配置项

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

### ReAct

- `ENABLE_REACT_AGENT`
- `REACT_MAX_STEPS`
- `REACT_ALLOW_WRITE_TOOLS`
- `REACT_TRACE_MODE`

### ReAct Skills

- `ENABLE_SKILLS`
- `SKILL_DIRS`
- `SKILL_AUTO_HINT_LIMIT`
- `SKILL_RESOURCE_MAX_BYTES`

### Web Tools

- `ENABLE_WEB_TOOLS`
- `WEB_SEARCH_PROVIDER`
- `WEB_SEARCH_ENDPOINT`
- `WEB_SEARCH_API_KEY`
- `WEB_FETCH_MAX_CHARS`

### 行为开关

- `ENABLE_EMOTIONAL_MEMORY`
- `ENABLE_NATURAL_SCHEDULER`

### 消息聚合

- `MESSAGE_AGGREGATE_IDLE_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_WINDOW_MS`
- `MESSAGE_AGGREGATE_MAX_MESSAGES`

## 14. 常见第一次启动问题

- 后端能启动但不回复：先确认 `TARGETID` 是否正确，并检查消息来源是否符合当前过滤规则。
- 启动时报 character config not found：确认 `CHARACTER` 对应的 `config/character/<name>.json` 存在。
- AI 不回复：先确认 `config/ai_profiles.json` 的 active profile、`aiKey`、`aiBaseUrl` 和 `aiModel`。
- `/readyz` 返回失败：通常是 `DATA_DIR` 或 `LOG_DIR` 不可写。
- 前端空白或接口报错：先确认后端已启动，并检查 `HttpPort`。
- ReAct 工具没有写入状态：确认 `ENABLE_REACT_AGENT=true`，并在需要副作用时设置 `REACT_ALLOW_WRITE_TOOLS=true`；关闭时写工具对模型不可见。
- ReAct 没有注入 skill hints：确认 `ENABLE_SKILLS=true`、`SKILL_DIRS` 指向的目录存在，并且每个 skill 目录下有合法的 `SKILL.md`。
- ReAct 看不到 web tools：确认 `ENABLE_WEB_TOOLS=true`，Tavily API key 或 SearXNG endpoint 可用，并重启机器人。

更详细的排查说明见 [HELP.md](./HELP.md)。
