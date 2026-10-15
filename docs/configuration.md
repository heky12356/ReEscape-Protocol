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

管理服务默认监听 `127.0.0.1`。浏览器管理后台通过后端配置的单个 `ADMIN_API_KEY` 登录，不使用用户名体系或密码哈希。这个密钥按当前设计保留为后端明文配置，只能放在服务器 `.env`、进程环境或部署平台的私有 Secret 中，不得放进 `VITE_*` 变量、前端静态文件、配置响应和日志。

```env
ADMIN_LISTEN_HOST=127.0.0.1
ADMIN_API_KEY=请替换为随机长密钥
ADMIN_CORS_ORIGINS=https://admin.example.com
ADMIN_COOKIE_SECURE=true
```

登录成功后后端签发 `admin_session` HttpOnly、SameSite=Lax Cookie，会话默认有效期 8 小时并保存在进程内存中。服务重启或 `ADMIN_API_KEY` 轮换后，已有会话失效。CLI 等兼容客户端仍可使用 `Authorization: Bearer <ADMIN_API_KEY>`，浏览器主流程不再保存 Bearer 密钥。

登录失败按客户端 IP 限流；短时间连续失败会暂时返回 `429`。管理写请求会校验 `Origin`，CORS 只允许显式配置的来源，不能配置 `*`。生产环境应通过 HTTPS 反向代理暴露管理后台，并确保 `DATA_DIR`、`LOG_DIR` 具备最小写权限。

开发环境使用 HTTP 时可设置 `ADMIN_COOKIE_SECURE=false`；浏览器通过 HTTPS 访问时（包括 TLS 在反向代理终止）必须设置 `ADMIN_COOKIE_SECURE=true`。

## 运行时重载

管理后台更新配置后会执行一次事务式重载：系统先读取环境变量、AI profile、角色和 Skill 配置，构造候选配置并完成范围校验；全部通过后才替换运行中的配置。解析失败、校验失败、角色或 Skill 加载失败时，旧配置和旧 Skill 索引保持不变。

每次成功重载都会递增配置版本，并记录更新时间和变更作用域。作用域包括 `bot`、`model`、`skill`、`logging`、`runtime` 和 `storage`，可在管理后台配置响应的 `configVersion`、`configUpdatedAt` 和 `configReloadScopes` 字段中查看。

配置 callback 在候选配置完成校验后执行，并带有本次版本和变更作用域；callback 失败会使本次重载回滚。callback 不应执行长时间阻塞操作，也不应把密钥写入日志。

## 持久化配置与重启

```env
PERSISTENCE_BACKEND=file
DATA_DIR=./data
SQLITE_PATH=./data/runtime.sqlite
SQLITE_BACKUP_DIR=./data/backups
SQLITE_BUSY_TIMEOUT_MS=5000
```

`file` 为默认后端；显式选择 `sqlite` 后使用 SQLite 快照、事件和可查询读模型。这些存储配置在启动时建立活动 Store。运行时尝试修改上述任一设置会拒绝本次重载，并提示需要进程重启；旧配置、配置版本、活动 Store 和重载 callback 保持不变。原有自定义 SQLite 路径不会被无关重载重置。

修改部署配置后，应先确认数据迁移与备份方案，再重启进程；切换后端不会自动迁移原有 JSON。`SQLITE_BACKUP_DIR` 当前只是目录配置，不代表已有完整数据库备份/恢复工具。

认证的 `/api/admin/persistence` 返回实际运行后端、schema 与查询能力，前端应据此判断可用功能；完整契约见[运维与数据查询](./operations.md)。
