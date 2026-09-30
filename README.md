# ReEscape Protocol

ReEscape Protocol 是一个基于 OneBot WebSocket 的聊天机器人运行时，目标是把“消息接入、上下文管理、角色提示词、ReAct 工具调用、长期记忆、自然调度和管理后台”拆成清晰边界。

当前主链路已经从“固定分类器 + 回复函数”过渡到“Agent Runtime + Tool Registry”的结构：模型在需要时可以通过工具读取记忆、更新好感、查询会话状态、获取时间上下文或选择图片素材。

## 系统概览

- `cmd/bot`：程序入口，只负责启动主进程。
- `internal/app`：应用装配层，负责连接配置、存储、入站管线、Agent、调度器和管理后台。
- `internal/connect`：OneBot WebSocket/API 连接和消息发送。
- `internal/inbound`：入站消息管线，负责聚合、去重、过滤和标准化。
- `internal/agent`：ReAct Agent Runtime，负责系统提示词、上下文、工具循环、预算、trace 和结果归一。
- `internal/tools`：工具注册、执行、schema 和策略；当前包含记忆、好感、图片素材、会话状态、主动触达计划和时间工具。
- `internal/domain`：业务领域逻辑，例如好感系统。
- `internal/state`：按用户/会话维护状态、对话历史和上下文层。
- `internal/memory`：长期画像、事实记忆和情绪记忆。
- `internal/service`：共享服务与遗留回复逻辑；部分旧分类和回复辅助逻辑仍在这里，后续应继续收敛到 Agent/Tools/Domain 边界。
- `internal/config`：环境变量、AI profiles、Character Card 和 prompt section 组合。
- `internal/admin`：管理后台 API、健康检查、指标、配置和日志接口。
- `web/`：管理后台前端。

## 主要能力

- OneBot WebSocket 收发消息。
- 连续碎片消息聚合。
- 按用户/会话隔离上下文和状态。
- ReAct 工具调用，可按需读取或写入业务状态。
- ReAct Skills：兼容标准 `skill-name/SKILL.md` 包，由 runtime 解析候选并自动加载高置信度 skill，扩展资源按需读取。
- 长期记忆：用户画像、事实记忆、情绪状态。
- 好感系统：领域逻辑在 `internal/domain/affection`，工具入口在 `internal/tools/affection`。
- 图片素材回复：素材索引在 `assets/images/index.json`。
- Character Card：角色身份、语气、边界和示例回复来自 `config/character/*.json`。
- AI profile：模型、base URL、key、temperature、token、timeout 等模型参数来自 `config/ai_profiles.json`。
- 自然定时发送。
- Web 管理后台、结构化日志、健康检查、就绪检查和 Prometheus 指标。

## 配置边界

项目现在有三类配置源，建议按下面的职责使用：

- `.env`：运行环境和选择器。用于连接 OneBot、选择当前 AI profile、选择当前 Character Card、功能开关、日志和数据目录。
- `config/ai_profiles.json`：AI 模型主配置。该文件通常由 `config/ai_profiles.example.json` 复制生成，并被 `.gitignore` 忽略。
- `config/character/*.json`：Character Card。`CHARACTER=default` 会加载 `config/character/default.json`。

AI 参数的优先级：

1. `AI_CONFIG_FILE` 决定读取哪个 profile 文件，默认是 `./config/ai_profiles.json`。
2. `AI_PROFILE` 决定使用 profile 文件里的哪个 profile。
3. active profile 中的 `aiBaseUrl`、`aiModel`、`aiKey`、`aiTemperature` 等字段会写入运行时配置。
4. `.env` 中的 `AI_KEY`、`AI_BASEURL`、`AI_MODEL`、`AI_TEMPERATURE` 等仍然有效，但主要作为首次生成 profile 或 profile 缺失时的兜底值。

因此，日常修改模型配置时优先改 `config/ai_profiles.json` 或通过管理后台修改；`.env` 主要保留连接、选择器和功能开关。

## 常用配置项

- 连接：`HOSTADD`、`WsPort`、`HttpPort`、`Token`、`TARGETID`
- AI profile：`AI_PROFILE`、`AI_CONFIG_FILE`
- AI fallback：`AI_KEY`、`AI_BASEURL`、`AI_MODEL`
- 角色：`CHARACTER`、`ALLOW_CHARACTER_IDENTITY_EXPLANATION`
- ReAct：`ENABLE_REACT_AGENT`、`REACT_MAX_STEPS`、`REACT_TOOL_TIMEOUT_MS`、`REACT_ALLOW_WRITE_TOOLS`、`REACT_TRACE_MODE`
- ReAct skills：`ENABLE_SKILLS`、`SKILL_DIRS`、`SKILL_CANDIDATE_MIN_SCORE`、`SKILL_AUTO_LOAD_MIN_SCORE`、`SKILL_AUTO_LOAD_MIN_CONFIDENCE`、`SKILL_MAX_AUTO_LOADED`、`SKILL_RESOURCE_MAX_BYTES`
- ReAct web tools：`ENABLE_WEB_TOOLS`、`WEB_SEARCH_PROVIDER`、`WEB_SEARCH_ENDPOINT`、`WEB_SEARCH_API_KEY`、`WEB_SEARCH_MAX_RESULTS`、`WEB_TOOL_TIMEOUT_MS`
- 聚合：`MESSAGE_AGGREGATE_IDLE_WINDOW_MS`、`MESSAGE_AGGREGATE_MAX_WINDOW_MS`、`MESSAGE_AGGREGATE_MAX_MESSAGES`
- 主动触达保护：`PROACTIVE_CLAIM_LEASE_MS`、`PROACTIVE_SKIP_ON_PENDING_USER`、`PROACTIVE_USER_MESSAGE_GRACE_MS`
- 上下文：`CONTEXT_RECENT_TURNS`、`CONTEXT_SUMMARY_MAX_TURNS`、`CONTEXT_OPEN_LOOP_LIMIT`
- 图片：`ENABLE_VISION_INPUT`、`ENABLE_IMAGE_ASSET_REPLY`、`IMAGE_ASSET_DIR`、`IMAGE_ASSET_INDEX_FILE`
- 运行：`DATA_DIR`、`LOG_DIR`、`LOG_LEVEL`、`LOG_FORMAT`
- 调试：`ENABLE_AI_RAW_LOG` 会把 AI 原始请求/响应写入 `LOG_DIR/ai_raw_YYYY-MM-DD.log`

- `ENABLE_SPACE_SEGMENT_DELIMITER`
  - 默认 `false`。
  - 关闭时仅把 `$` 识别为回复分段符。
  - 开启时仍优先 `$`，但如果模型使用空格分段，发送链路也会按空格拆段。

## 对话链路

```text
OneBot event
  -> internal/app receiver
  -> internal/inbound aggregate/dedupe/filter/normalize
  -> internal/app turn processor
  -> state + memory + prompt sections
  -> internal/agent ReAct runtime
  -> internal/tools registry/executor
  -> connect outbound send
```

ReAct Runtime 是唯一的消息处理链路。关闭 `ENABLE_REACT_AGENT` 时，系统会明确报告运行时不可用，不会静默切换到旧 handler/service 路径。

## ReAct Skills

Skills 是 ReAct 的只读策略层，用来指导某类任务或回复场景，不替代 Character Card、记忆、状态或工具权限。项目兼容标准 Agent Skill 包格式：

```text
skill-name/
  SKILL.md
  references/
  assets/
  scripts/
```

`SKILL.md` 必须包含 YAML frontmatter，至少提供 `name` 和 `description`：

```markdown
---
name: comfort
description: Use when the user expresses sadness, stress, fatigue, disappointment, or emotional overwhelm.
---

# Comfort

先承接用户情绪，不急着解决问题。
```

默认从 `./config/skills` 加载。metadata 只用于候选召回：高置信度 skill 由 runtime 自动加载完整 `SKILL.md` 并作为 `Activated Skills` 注入当前 turn；中置信度结果只作为 `Skill Candidates` 注入，模型确认适用后必须先调用 `read_skill`。`references/`、`assets/` 和 `scripts/` 中的文本仍通过 `read_skill_resource` 渐进读取。

同一个 ReAct turn 只执行一次匹配和激活，默认最多自动加载 1 个 skill。普通寒暄或只有通用 description 词的弱匹配不会注入 skill 正文。`read_skill` 主要用于中置信度候选和 `search_skills` 新发现的 skill，不再承担主要激活职责。

相关配置：

```env
ENABLE_SKILLS=true
SKILL_DIRS=./config/skills
SKILL_AUTO_HINT_LIMIT=3
SKILL_CANDIDATE_MIN_SCORE=8
SKILL_AUTO_LOAD_MIN_SCORE=16
SKILL_AUTO_LOAD_MIN_CONFIDENCE=0.75
SKILL_MAX_AUTO_LOADED=1
SKILL_FORCE_READ_ON_CANDIDATE=false
SKILL_RESOURCE_MAX_BYTES=65536
SKILL_ALLOW_SCRIPTS=false
SKILL_LOAD_SYSTEM=false
```

`search_skills`、`read_skill`、`read_skill_resource` 都是只读工具，即使 `REACT_ALLOW_WRITE_TOOLS=false` 也会保留。第一阶段不会执行 `scripts/`，只允许把其中的文本作为资源读取。

## ReAct Web Tools

`web_search` / `web_fetch` 是 ReAct 只读工具，默认通过 `ENABLE_WEB_TOOLS=false` 关闭。搜索 provider 支持 `searxng` 和 `tavily`；抓取使用本地 Go HTTP client，并带有 SSRF 防护、文本 content-type 限制、大小限制和超时。

需要稳定 agent 搜索时推荐 Tavily：

```env
ENABLE_REACT_AGENT=true
ENABLE_WEB_TOOLS=true
WEB_SEARCH_PROVIDER=tavily
WEB_SEARCH_API_KEY=tvly-xxx
WEB_SEARCH_MAX_RESULTS=5
```

`searxng` 适合自建免费搜索，实例需要启用 JSON search API，并把 endpoint 配到 `/search`。公开 SearXNG 实例可能返回 HTML bot challenge，不建议依赖。

`tavily` 适合稳定 agent 搜索。`WEB_SEARCH_ENDPOINT` 可留空，默认使用 `https://api.tavily.com/search`；`WEB_SEARCH_API_KEY` 必填。`ENABLE_WEB_TOOLS`、provider、endpoint、API key 这类工具注册配置修改后建议重启机器人；运行时保存后会热重载配置值，但已构建的 registry 不会自动重建。

## ReAct 记忆写入

`get_memory_context` 是只读工具，用于读取短期上下文、用户画像、事实记忆和情绪模式。`remember_fact`、`update_profile` 和 `update_affection` 是写工具，默认由 `REACT_ALLOW_WRITE_TOOLS=false` 关闭。

关闭写工具时，它们不会暴露给模型，也不会出现在 ReAct system prompt 的可用工具列表里。开启 `REACT_ALLOW_WRITE_TOOLS=true` 后，prompt 会注入 Memory Tool Rules，引导模型在用户明确说“记住”“以后”“我喜欢”“我叫”“不要再”等长期偏好或事实表达时调用写入工具。

写入规则保持保守：只记录用户明确表达的身份、地点、计划、重要关系、长期偏好或互动禁忌；不要记录一次性情绪、临时吐槽、普通寒暄、含糊猜测或模型推断。

## ReAct 主动触达计划

ReAct 模式下，agent 可以通过 `get_proactive_schedule` 查询当前 session 的下一次主动触达时间、手动计划摘要和 meta。开启 `REACT_ALLOW_WRITE_TOOLS=true` 后，agent 还可以通过 `update_proactive_schedule` 根据用户明确约定设置、推迟或取消下一次主动触达。

主动触达计划只写入 session state，不直接持有调度器实例。自然调度器仍负责 sweep 和到点触发；当 ReAct 已显式管理 schedule 时，普通自动重排不会覆盖手动计划。

为避免“旧主动计划过期后，用户刚发消息又同时触发 proactive 回复”的竞态，可以配置主动触达保护：

```env
PROACTIVE_CLAIM_LEASE_MS=120000
PROACTIVE_SKIP_ON_PENDING_USER=true
PROACTIVE_USER_MESSAGE_GRACE_MS=30000
```

- `PROACTIVE_CLAIM_LEASE_MS`：主动触达 turn 的 claim 租约时长，避免 ReAct 调用失败后永久占用。
- `PROACTIVE_SKIP_ON_PENDING_USER`：当同一 session 已有用户消息 pending 或正在处理时，跳过本次 proactive 发送。
- `PROACTIVE_USER_MESSAGE_GRACE_MS`：用户消息进入系统后的保护窗口，用于覆盖消息聚合和刚结束处理的短暂竞态。

推荐保持 `PROACTIVE_SKIP_ON_PENDING_USER=true`。用户消息触发的 ReAct 回复完成后，会按当前逻辑重新安排下一次自动主动触达；如果本轮 ReAct 调用了 `update_proactive_schedule`，则以工具设置的计划为准。

## 上下文记忆

会话完整 transcript 保存在 `Session.Conversation`，作为持久化和调试的事实来源，不在每次 AI 调用时完整塞入 prompt。

AI 调用会使用分层、有界的上下文：

- 最近上下文按 user turn 裁剪，而不是按原始消息条数裁剪。
- 较早对话压缩为 deterministic rolling summary。
- 未闭合问题、待跟进承诺、用户画像、事实记忆和情绪状态会单独注入 prompt。
- ReAct 模式下，模型也可以通过工具主动读取或更新相关状态。
- `CONTEXT_RECENT_TURNS`、`CONTEXT_SUMMARY_MAX_TURNS`、`CONTEXT_OPEN_LOOP_LIMIT` 控制 prompt 体积。

## 快速开始

见 [QUICKSTART.md](./QUICKSTART.md)。

## 帮助

见 [HELP.md](./HELP.md)。

## Web 管理后台

前端说明见 [web/README.md](./web/README.md)。
