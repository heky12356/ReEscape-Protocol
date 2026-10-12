# 运维数据传输

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
GET /api/admin/state/{kind}?user_id=42&session_id=private:42&status=pending&limit=100
```

`kind` 支持 `sessions`、`intents`、`open_loops` 和 `deliveries`。查询只读取 SQLite 索引和 payload，不重新加载完整 JSON 快照。文件后端不会伪造 SQLite 读模型，会返回不可用状态。
