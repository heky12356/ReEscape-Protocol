# 架构概览

消息主链路如下：

```text
OneBot event
  -> internal/app receiver
  -> inbound aggregate / dedupe / filter / normalize
  -> state + memory + prompt sections
  -> internal/agent ReAct runtime
  -> internal/tools registry / executor
  -> internal/connect outbound send
```

## 主要模块

- `internal/app`：应用装配、生命周期和连接监督
- `internal/connect`：OneBot WebSocket/API 连接与发送
- `internal/inbound`：消息聚合、去重、过滤和标准化
- `internal/agent`：ReAct 循环、上下文、预算、trace 和结果归一
- `internal/tools`：工具注册、schema、执行和权限策略
- `internal/state`、`internal/memory`：会话状态、对话历史和长期记忆
- `internal/config`：环境变量、AI profile、角色和 prompt 组合
- `internal/admin`：配置、日志、健康检查和指标接口
- `web`：管理后台前端

ReAct Runtime 是消息处理的唯一主链路。关闭 `ENABLE_REACT_AGENT` 时，系统会明确报告运行时不可用，不会静默回退到旧 handler 路径。
