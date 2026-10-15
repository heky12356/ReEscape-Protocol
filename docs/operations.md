# 运维与数据查询

管理后台提供受认证保护的异步文件导出、上传预检查和 SQLite 备份接口。ZIP 文件传输与 SQLite 整库备份是两种不同产物；实际数据恢复通过停机 CLI 执行。

ZIP 范围是 `DATA_DIR` 中允许传输的运行数据文件，排除 `exports`、`imports`、备份目录、`.maintenance`、`.env*`、管理会话、进程锁和数据库文件。自定义 `SQLITE_PATH`、其 WAL/SHM/journal 及 `SQLITE_BACKUP_DIR` 也受保护，不受数据库文件名或是否位于 DATA_DIR 内影响。

## 导出

创建导出任务：

```http
POST /api/admin/transfers/export
```

接口返回 Job。使用返回的 `id` 查询状态：

```http
GET /api/admin/transfers/{id}
```

状态变为 `completed` 后下载：

```http
GET /api/admin/transfers/{id}/download
```

导出 ZIP 包含版本化 `manifest.json`，每个文件记录相对路径、大小和 SHA-256。导出和压缩过程在后台执行，客户端应轮询 Job 状态。

## 上传与导入预检查

以 multipart 字段 `archive` 上传导出 ZIP：

```http
POST /api/admin/transfers/import
Content-Type: multipart/form-data
```

该接口返回 `kind: "import_validation"` 的异步 Job。服务端限制压缩包和解压总大小为 128 MiB、文件数为 4096、manifest 为 1 MiB，并检查版本、规范相对路径、重复项、大小、SHA-256、符号链接与受保护路径。

**`completed` 只表示预检查通过，`applied` 始终为 false，不表示数据已恢复。** 验证成功的包保留在 `DATA_DIR/imports/.upload-<job-id>.zip`，`nextAction` 提示后续停机命令；失败包删除。服务重启后内存 Job 会失效，已暂存的包不会自动执行。需要恢复时按下述 CLI 流程操作。

运行中的 ZIP 导出可能跨越多次刷盘，不承诺跨文件的同一时间点快照；SQLite 数据应使用整库备份。

## SQLite 整库备份

```http
POST /api/admin/persistence/backup
```

SQLite 后端返回 `kind: "sqlite_backup"` 的异步 Job。继续使用 `/api/admin/transfers/{id}` 查询状态，完成后通过 `/api/admin/transfers/{id}/download` 下载数据库文件，类型为 `application/octet-stream`。文件后端返回 501；未认证不可创建或下载，Cookie 写请求仍要求合法 Origin。

备份使用 [SQLite VACUUM INTO](https://www.sqlite.org/lang_vacuum.html) 生成一致的数据库副本，包含已提交到 WAL 的数据、快照、事件、cursor、schema 和派生表。它不包含尚未刷盘的内存修改，也不会修改原数据库的逻辑数据或重发消息。

备份先生成私有临时文件，完成 integrity/schema/JSON 校验与文件同步后才发布；拒绝覆盖已有同名文件。默认产物保存在 `SQLITE_BACKUP_DIR`，单份上限 4 GiB。失败 Job 不提供可下载的半成品；备份会占用 SQLite 单连接，应在低流量时执行。

## 离线迁移与恢复 CLI

在仓库根目录使用 `go run ./cmd/datactl`，也可先构建独立工具。该工具不加载 `.env` 或 AI profile，不启动 OneBot、Provider、Manager 或后台刷盘。参数必须与机器人实际配置一致，包括自定义数据库和备份路径。

除只读 `validate-sqlite` 外，这些命令会获取与机器人相同的数据目录/数据库进程锁；机器人运行时拒绝执行。应先停止机器人并等待最终刷盘和进程退出，关闭外部数据库编辑器，再操作。锁文件保留是正常现象，不要删除它来绕过锁。

### 将已有 JSON 迁入新数据库

```shell
go run ./cmd/datactl migrate-json --data-dir ./data --sqlite-path ./data/runtime.sqlite
```

仅读取已知 session、Intent/OpenLoop、用户画像、事实记忆、情绪记忆、好感和事件快照，单个源文件上限 128 MiB。支持有效 `.bak` 回退和旧 `conversations` 格式；源 JSON 保持不变。事件 cursor 保留或按旧文件规则补齐，缺失 ID 使用确定性导入 ID，原事件关联和大整数 payload 保留。

目标数据库必须不存在；即使已有文件为空也拒绝。所有数据先在私有数据库中导入、验证，再一次性发布。重复执行不会重复写事件；中途失败不发布目标，也不自动切换后端。成功后再修改 `PERSISTENCE_BACKEND=sqlite` 并启动机器人，确认状态和事件后保留原 JSON 作为回退材料。

### 备份与验证

```shell
go run ./cmd/datactl backup-sqlite --data-dir ./data --sqlite-path ./data/runtime.sqlite --output ./data/backups/manual-20261015.sqlite
go run ./cmd/datactl validate-sqlite --source ./data/backups/manual-20261015.sqlite
```

CLI 输出 JSON 报告，包含 schema 版本、快照/事件数量、大小与 SHA-256。验证只接受已知连续迁移版本及完整的独立数据库，不执行迁移；拒绝损坏、未来 schema、结构不匹配、空/超大文件和附带 WAL/SHM/journal 的输入。新备份/迁移发布需要目标目录位于支持硬链接的本地文件系统（如 NTFS、ext4）；不支持时会失败，不覆盖已有文件。

### 恢复 SQLite

```shell
go run ./cmd/datactl restore-sqlite --data-dir ./data --sqlite-path ./data/runtime.sqlite --source ./data/backups/manual-20261015.sqlite
```

工具先复制并验证备份，在私有候选库上升级已知旧 schema，再替换目标。原备份不修改；不兼容版本或校验失败不会覆盖目标。已有数据库会先保留为同目录的 `*.pre-restore-*`，报告通过 `rollbackPath` 给出路径，可在再次停机后用作恢复来源。替换以单文件方式发布，不要求原目标数据库本身健康。

目标存在 WAL/SHM/journal 时拒绝替换，不能直接删除这些文件。对正常可读的旧库，可在确认无运行进程后执行 `backup-sqlite`，其数据库关闭流程会处理 WAL；仍存在 sidecar 或库损坏时应先保留完整原文件集并排查，不覆盖未处理的事务。

### 恢复 ZIP 文件数据

```shell
go run ./cmd/datactl restore-files --data-dir ./data --sqlite-path ./data/runtime.sqlite --backup-dir ./data/backups --archive ./data/imports/.upload-import-example.zip
```

将示例 archive 路径替换为实际上传包或独立导出 ZIP。命令仅适用于 file 后端数据恢复：按 manifest 覆盖列出的文件，不删除未列出的文件；SQLite 恢复应使用 `restore-sqlite`。它会再次验证全部包内容，暂存所有文件并复制原文件后，才开始替换。

恢复目录 `DATA_DIR/.maintenance/restore-*` 保留源包、旧文件和 journal，报告通过 `rollbackDirectory` 返回位置。中途失败会回滚整组文件；进程中断时，下一次维护或机器人启动会先回滚 `prepared` 记录，再加载 Manager。journal 损坏、旧文件缺失或回滚失败会阻止启动，应保留该目录排查，不通过删除 journal 跳过恢复。

恢复成功后启动机器人，重新登录并检查会话、记忆、事件和未完成任务。旧 Worker 已随进程停止，不会继续刷盘覆盖恢复数据。备份/预检查本身不触发消息投递；恢复后重启会恢复正常调度行为。

## SQLite 读模型查询

启用 `PERSISTENCE_BACKEND=sqlite` 后，管理后台可以查询快照同步得到的 session、Intent、OpenLoop，以及由事件投影得到的 Delivery：

```http
GET /api/admin/state/intents?user_id=42&session_id=private:42&status=pending&limit=100
```

路径中的模型类型支持 `sessions`、`intents`、`open_loops`（兼容 `open-loops`）和 `deliveries`。查询不重新加载完整 JSON 快照。

| 参数 | 说明 |
|---|---|
| `user_id` | 非负整数；省略或 `0` 表示不按用户筛选 |
| `session_id` | Session 查询按记录 ID 匹配；其余模型按关联会话匹配 |
| `status` | Intent/OpenLoop 使用领域状态；Delivery 使用业务投递状态；Session 不支持此参数 |
| `limit` | 默认 100，允许 1–200；超出范围返回 400 |
| `cursor` | 上页返回的 `nextCursor`；使用相同模型类型、筛选条件与 limit |

未知、重复、空参数、非法 cursor 或不支持的筛选返回 `400`。文件后端返回 `501`；尚未初始化或已关闭的持久化实例返回 `503`。

返回格式示例：

```json
{
  "kind": "sessions",
  "records": [],
  "limit": 100,
  "refreshedAt": "2026-10-15T00:00:00.000000000Z"
}
```

有下一页时返回 `nextCursor`。列表按固定精度 UTC 更新时间、唯一 ID 降序排列。cursor 绑定读模型版本；翻页期间有快照同步、投递投影更新或重建时，返回 `409` 和 `code: "read_model_changed"`，客户端应丢弃旧分页结果，从第一页重新查询。此接口不持有跨请求的数据库快照。

Session 列表的 `payload` 是预先保存的概要，不含 `conversation`。其他列表仍返回对应模型的 payload。按需读取完整单条记录：

```http
GET /api/admin/state/sessions/private%3A42
```

详情接口适用于四类模型，不接受查询筛选参数；不存在的记录返回 `404`。Session 详情仍包含完整 conversation，逐条聊天分页和 WebUI 查询页尚未接入。

## 实际持久化状态

```http
GET /api/admin/persistence
```

接口通过应用当前持有的 Persistence 实例报告 `backend`、`ready`、`schemaVersion`、分页上限、`capabilities`，以及各模型支持的 `filters` 和 `refreshedAt`。不返回数据库绝对路径，不通过环境配置推断已运行的后端。文件后端 `readModels` 为空，SQLite 当前 schema 为 3。

`refreshedAt` 表示查询投影的最近同步时间，并非最近聊天时间。Session/Intent/OpenLoop 查询反映已保存或已加载的状态快照，可能落后于尚未 flush 的内存状态。缺失旧记录的更新时间保留为零时间，不在启动时伪造业务更新时间。

## Delivery 投影与重建

Delivery 查询的 `status` 使用 `sending`、`delivered`、`partial`、`failed`、`cancelled`，`eventType` 保留原事件类型，`sessionId`、`intentId`、`turnId` 保留关联信息。Intent 结果和单元投递事件不覆盖整轮投递终态。

终态优先于 started；同一等级按事件时间、持久化 cursor 确定先后，因此晚到的开始事件或旧终态不会覆盖较新的终态。历史 `delivery_sent` 兼容映射为 `delivered`。事件事实仍完整保存。

SQLite 追加同 ID、相同内容的事件是幂等操作；同 ID、不同内容使整批追加失败并回滚。重试应保留原 ID、时间与内容，查询回传的 cursor 不参与内容比较。事件 cursor 保证递增，但不要求连续。

升级到 schema 3 时会在事务中修复旧时间排序键、生成 Session 概要，并从已有 events 重建 Delivery 投影。之后可显式重建：

```http
POST /api/admin/state/deliveries/rebuild
```

该接口受管理认证和 Cookie Origin 校验保护，仅重建派生表，不修改事件、调用模型或重发消息。重建失败回滚已有投影；成功重建会使旧 Delivery 分页 cursor 失效。大量事件的重建会占用当前 SQLite 单连接，应安排在低流量窗口。
