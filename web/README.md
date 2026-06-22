# ReEscape Admin Web

这是项目的管理后台前端，负责查看运行状态、管理 AI 配置、维护 Prompt 和查看日志流。

## 页面结构

- `总览`：查看系统状态、健康检查、配置摘要和运行概况
- `AI 配置`：切换 AI profile，查看和调整模型参数
- `人格与 Prompt`：管理角色文件和最终注入模型的 Prompt
- `日志流`：查看实时日志、历史日志和关键运行信息

## 开发启动

```bash
cd web
npm install
npm run dev
```

开发服务器默认运行在 `http://localhost:5173`，并将 `/api` 代理到后端管理服务。

## 生产构建

```bash
cd web
npm run build
```

构建产物会输出到 `web/dist`，由 Go 管理后台直接托管。

## 预览构建结果

```bash
cd web
npm run preview
```

## 常见依赖

- 后端管理服务默认端口：`8088`
- 配置接口：`/api/admin/config`
- 健康检查：`/healthz`
- 就绪检查：`/readyz`
- 指标：`/metrics`

## 目录说明

- `src/App.jsx`：页面总入口和导航
- `src/pages/`：各功能页面
- `src/components/`：通用组件和布局
- `src/api/`：前端接口封装
- `src/hooks/`：前端数据拉取与状态管理
