# 开发指南

## 本地检查

提交前运行：

```powershell
gofmt -l .
go vet ./...
go test ./...
Set-Location web
npm ci
npm run build
Set-Location ..
npm ci
npm run docs:build
```

Linux CI 还会运行 `go test -race ./...`。

## 目录约定

生产代码位于 `cmd/`、`internal/` 和 `web/`。公开用户文档位于 `docs/`，开发过程中的路线图和内部记录位于 `doce/`，后者不应提交到公开仓库。

新增功能应优先放入现有模块边界，跨模块行为使用临时目录、内存对象或 `httptest.Server` 测试，不依赖本机数据、日志或真实 API key。

## 提交规范

提交信息使用中文 Conventional Commits，例如：

```text
feat: 增加 OneBot 断线重连保护
fix: 修复配置重载失败时的状态回滚
test: 补充 Intent claim 并发测试
docs: 更新公开部署说明
```

详细贡献流程见仓库根目录的 `CONTRIBUTING.md`。
