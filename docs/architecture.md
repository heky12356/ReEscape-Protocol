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

SQLite 的 `snapshots` 保留 Manager 的完整状态快照；Session/Intent/OpenLoop 与聊天消息是派生读模型，Delivery 来源于事实事件。schema 4 的 `conversation_messages` 按会话与序号索引，消息分页独立于完整 Session payload；`conversation_revisions` 记录单个会话聊天内容版本。重复保存不重复消息，删除会话会清理派生聊天。

WebUI 的“会话与状态”先读取活动 Persistence 能力，再查询分类列表、概要、详情和消息页；请求切换与卸载会取消旧查询，401 接入现有登录失效流程。聊天只展示已保存的 user/assistant 事实，默认不加载远端附件或展开原始 payload。file 后端提供明确的能力提示，继续使用原持久化方式。

ReAct Runtime 是消息处理的唯一主链路。关闭 `ENABLE_REACT_AGENT` 时，系统会明确报告运行时不可用，不会静默回退到旧 handler 路径。

## 事件与扩展 Hook

`internal/eventlog` 保存事实事件，并通过统一关联字段连接一次请求的各个阶段：

- `request_id`、`turn_id`、`session_id`：请求和会话上下文
- `intent_id`、`delivery_id`：主动任务和真实投递结果
- `tool_name`、`tool_call_id`、`tool_provider`、`tool_origin`、`tool_version`、`provider`、`model`：工具及模型来源

旧事件中的 `tool` 字段仍可读取，新事件同时写入 `tool_name`。投递关联字段位于事件顶层，`data` 只保留业务细节。

事件存储和扩展通知通过 `HookedStore` 分层：blocking Hook 在事实写入前串行执行，可以返回错误阻止写入；observe Hook 在写入成功后异步执行，超时、错误和 panic 会被隔离，不会改变事实日志。

观测指标通过 Prometheus 文本接口暴露。工具指标包含 `tool`、`provider`、`origin` 和结果标签，并记录调用次数与耗时；AI 指标按请求类型和模型记录成功/失败与耗时；Delivery 指标记录投递状态、主动触达维度和每个消息单元的结果。

模型调用经过 `internal/llm` 的 ModelTask、Provider 和 Orchestrator 边界。当前 Agent 使用 `reply` 任务；`memory` 与 `planner` 已预留独立模型配置，分别由 `AI_MEMORY_MODEL` 和 `AI_PLANNER_MODEL` 指定，未配置时回退到 `AI_MODEL`。

Tool Call 与 Tool Result 在发送给模型前、收到模型响应后和执行结果落地前进行关联校验，拒绝孤儿结果、重复或乱序 call ID。模型调用失败会生成脱敏 `FailureSnapshot` 和 `llm_request_failed` 事件；快照只保留结构与摘要哈希，`ReplayRequest` 生成占位请求，不会自动调用 Provider 或触发消息投递。
