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

至少确认 `HOSTADD`、`WsPort`、`HttpPort`、`Token` 和 `TARGETID`。生产环境应设置随机的 `ADMIN_API_KEY`，并将管理后台限制在可信来源：

```env
ADMIN_LISTEN_HOST=127.0.0.1
ADMIN_API_KEY=请替换为随机长密钥
ADMIN_CORS_ORIGINS=https://admin.example.com
ADMIN_COOKIE_SECURE=true
```

`ADMIN_API_KEY` 是后端明文配置，只用于登录校验；它不会进入前端构建产物或浏览器存储。浏览器登录成功后使用有效期 8 小时的 HttpOnly 会话 Cookie，后端重启后需要重新登录。开发环境没有 HTTPS 时可使用 `ADMIN_COOKIE_SECURE=false`，并把 `ADMIN_CORS_ORIGINS` 改成实际前端地址。

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

生产环境建议通过 HTTPS 反向代理暴露管理后台，Go 服务只监听 `127.0.0.1:8088`。例如 Nginx：

```nginx
server {
    listen 443 ssl http2;
    server_name admin.example.com;

    ssl_certificate     /etc/letsencrypt/live/admin.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/admin.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8088;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

生产 CORS 只配置 `https://admin.example.com` 这类明确来源，不要使用 `*`。防火墙也不应直接开放后端 `8088`。

## 开发前端

```powershell
Set-Location web
npm run dev
```

前端开发服务器默认运行在 `http://localhost:5173`，并将 `/api` 请求代理到后端管理服务。
