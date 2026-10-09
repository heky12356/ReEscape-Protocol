# 配置参考

运行配置来自 `.env`、`config/ai_profiles.json` 和 `config/character/*.json` 三类文件。

## 连接和运行

- `HOSTADD`、`WsPort`、`HttpPort`、`Token`：OneBot 与管理服务连接参数
- `TARGETID`：机器人主要响应的目标会话
- `DATA_DIR`、`LOG_DIR`、`LOG_LEVEL`、`LOG_FORMAT`：数据、日志和日志格式
- `ONEBOT_RECONNECT_INITIAL_MS`、`ONEBOT_RECONNECT_MAX_MS`：断线重连退避范围
- `ONEBOT_HEARTBEAT_INTERVAL_MS`、`ONEBOT_READ_TIMEOUT_MS`：连接保活参数

## AI 和角色

`AI_CONFIG_FILE` 指向 profile 文件，`AI_PROFILE` 选择 active profile。profile 中维护 `aiBaseUrl`、`aiModel`、`aiKey`、温度、token 上限和超时等参数。

`CHARACTER` 选择 `config/character/<name>.json`。角色文件负责身份、语气、边界和示例回复，不承担工具权限策略。

## ReAct Agent

```env
ENABLE_REACT_AGENT=true
REACT_MAX_STEPS=4
REACT_TOOL_TIMEOUT_MS=3000
REACT_ALLOW_WRITE_TOOLS=false
REACT_TRACE_MODE=basic
REACT_TOTAL_TIMEOUT_MS=30000
```

`REACT_ALLOW_WRITE_TOOLS=false` 时，记忆、好感和主动触达等写工具不会暴露给模型。开启前应先在测试会话中验证副作用。

Skills 默认从 `./config/skills` 加载，可通过 `ENABLE_SKILLS`、`SKILL_DIRS` 和 `SKILL_RESOURCE_MAX_BYTES` 调整。

## Web Tools

`ENABLE_WEB_TOOLS=false` 时不注册 `web_search` 和 `web_fetch`。启用 Tavily 或自建 SearXNG 时，请将 API key 和 endpoint 只放入本地环境变量，不要写入仓库。

## 安全边界

管理 API 默认要求 `ADMIN_API_KEY`，管理服务默认监听 `127.0.0.1`。生产部署应设置 `ADMIN_CORS_ORIGINS` 白名单，并确保 `DATA_DIR`、`LOG_DIR` 具备最小写权限。
