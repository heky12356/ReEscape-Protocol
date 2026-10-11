# 安全报告

请不要在公开 issue 中发布 API key、访问令牌、用户消息、日志原文或可复现的生产凭据。

发现可能影响用户数据、管理 API、OneBot 连接或远程请求能力的安全问题时，请优先通过私下渠道联系维护者，并提供：

- 受影响的版本或提交
- 复现步骤和必要配置
- 影响范围和建议修复方向

在修复版本发布前，请避免公开披露未修复的细节。普通 bug 和功能建议可以通过 issue 提交。

## 管理后台部署

管理后台使用后端配置的单密钥登录。`ADMIN_API_KEY` 按当前设计以明文保存在后端 `.env`、进程环境或部署平台 Secret 中；这不表示它可以放入前端环境变量。它不会写入前端构建产物、浏览器 `localStorage`、配置响应、事件或请求日志。

浏览器登录成功后只接收 `admin_session` HttpOnly Cookie。Cookie 使用 SameSite=Lax，默认 8 小时后过期；后端重启或管理密钥轮换会使已有会话失效。管理 API 的写请求会校验 `Origin`，登录失败会按客户端 IP 限流。CLI 兼容客户端可以继续使用 Bearer 密钥，但应通过安全的进程环境或 Secret 注入。

生产部署建议：

```env
ADMIN_LISTEN_HOST=127.0.0.1
ADMIN_API_KEY=请替换为随机长密钥
ADMIN_CORS_ORIGINS=https://admin.example.com
ADMIN_COOKIE_SECURE=true
```

后端只监听本机，由 HTTPS 反向代理负责证书和域名转发：

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

`ADMIN_CORS_ORIGINS` 必须列出实际管理域名，禁止使用 `*`。防火墙只公开代理端口，不要直接公开后端 `8088`。

## 生产验收

部署后至少完成以下检查：

- 清空浏览器站点数据后打开管理域名，登录页可以使用管理密钥建立会话。
- 刷新页面、切换页面和打开日志流不需要重复输入密钥。
- 点击退出登录后，管理接口返回未认证，重新登录可以恢复访问。
- 等待会话过期或撤销 Cookie 后，管理接口不能继续读取或修改数据。
- 重启后端后旧 Cookie 失效，必须重新登录。
- 使用未配置的 Origin 访问写接口会被拒绝；配置的 HTTPS Origin 可以正常保存配置和角色。
- 浏览器开发者工具、前端静态资源、访问日志和配置响应中都没有出现 `ADMIN_API_KEY` 的实际值。
- `/healthz`、`/readyz`、管理页面和 OneBot 收发在代理转发后均正常。
