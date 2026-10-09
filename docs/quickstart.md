# 快速开始

## 环境要求

- Go `1.23.6` 或更高版本
- Node.js `20.19` 或更高版本（公开文档构建使用 VitePress 2）
- 可用的 OneBot WebSocket 服务端
- 一个 OpenAI-compatible AI 接口

## 初始化配置

```powershell
Copy-Item .env.example .env
Copy-Item config\ai_profiles.example.json config\ai_profiles.json
```

至少确认 `HOSTADD`、`WsPort`、`HttpPort`、`Token` 和 `TARGETID`。生产环境应设置随机的 `ADMIN_API_KEY`，并将管理后台限制在可信来源。

不要提交 `.env`、`config/ai_profiles.json` 或任何真实 API key。

## 安装和启动

```powershell
go mod download
Set-Location web
npm ci
npm run build
Set-Location ..
go run ./cmd/bot
```

默认管理后台地址为 `http://127.0.0.1:8088`。启动后可检查 `/healthz`、`/readyz` 和 `/metrics`。

## 开发前端

```powershell
Set-Location web
npm run dev
```

前端开发服务器默认运行在 `http://localhost:5173`，并将 `/api` 请求代理到后端管理服务。
