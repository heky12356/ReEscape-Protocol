# 贡献指南

感谢参与 ReEscape Protocol。提交代码前，请确认变更范围，并避免提交密钥、用户消息、运行日志、`data/`、`logs/` 或本地 `.env`。

## 开发环境

- Go `1.23.6+`
- Node.js `20.19+`
- 可用的 OneBot WebSocket 服务端和 OpenAI-compatible AI 接口（运行机器人时需要）

常用检查命令：

```powershell
go mod download
go test ./...
go vet ./...
Set-Location web
npm ci
npm run build
Set-Location ..
npm ci
npm run docs:build
```

## 提交变更

提交信息采用中文 Conventional Commits：

```text
feat: 增加连接重试策略
fix: 修复配置热重载回滚
test: 补充状态并发测试
docs: 更新部署说明
```

新增行为应配套关键路径测试。测试不得依赖真实 API key、个人数据或本机持久化目录。

## Pull Request

Pull Request 应说明问题、行为变化、测试命令和已知限制。涉及用户可见行为时，请同步更新 `README.md` 或 `docs/`。内部路线图和开发记录位于 `doce/`，不应加入公开提交。
