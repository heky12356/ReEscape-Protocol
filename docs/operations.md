# 运维与数据查询

管理后台提供受认证保护的异步导出和导入接口。传输范围是 `DATA_DIR` 中的运行数据文件；导出结果不会包含 `exports`、`imports`、`.env`、管理会话或运行中的 SQLite 文件。

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

## 导入

以 multipart 字段 `archive` 上传导出 ZIP：

```http
POST /api/admin/transfers/import
Content-Type: multipart/form-data
```

导入同样返回异步 Job。服务端会限制压缩包和解压后的总大小，并校验 manifest 版本、规范相对路径、重复文件、大小、SHA-256 和符号链接。所有文件通过暂存文件原子替换，路径穿越或受保护路径会使 Job 失败。

导入不会覆盖 `.env`、`exports`、`imports`、管理会话和 SQLite 运行文件。生产环境应在导入前保留独立数据库备份，并在低流量窗口执行。

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
