<div align="center">

# ReEscape Protocol

一个智能、自然的聊天机器人，具备情感记忆和自然交互能力

</div>

---

## ✨ 功能特性

### 🤖 智能回复系统
- **预设回复**：针对常见问候和对话的快速回复
- **情感分析**：AI驱动的情感识别，理解用户当前心情
- **意图识别**：智能判断用户的交流意图和需求
- **上下文对话**：支持长对话模式，保持对话连贯性
- **多层处理**：PresetHandler → EmotionHandler → AIHandler 的处理链

### 🧠 情感记忆系统
- **交互历史**：记录与用户的对话历史和情感变化
- **个性化回复**：基于历史交互调整回复风格
- **情感模式识别**：分析用户的情感模式，提供更贴心的回复
- **偏好学习**：学习用户的交流偏好，优化交互体验
- **对话模式分析**：识别"需要关怀"、"积极活跃"、"情绪波动"等模式

### ⏰ 自然定时系统
- **智能间隔**：根据时间段动态调整发送间隔
- **活跃时间**：在用户活跃时间增加互动频率（30-60分钟）
- **休息模式**：在休息时间减少打扰（2-4小时）
- **随机因子**：添加随机性，模拟真人行为
- **消息池**：日常消息、情感消息、问候消息的智能选择

### 🎭 角色扮演系统
- **角色配置**：JSON格式的角色配置文件
- **性格设定**：内外性格、行为特征的详细定义
- **回复风格**：针对陌生人和熟人的不同回复模式
- **动态Prompt**：基于角色配置自动生成AI提示词

### 🎯 状态管理
- **多状态支持**：空闲、长对话、忙碌、需要安慰、需要鼓励等状态
- **状态机模式**：清晰的状态管理，避免混乱的全局变量
- **线程安全**：支持并发访问，保证数据一致性
- **对话历史**：按用户ID存储对话历史，支持上下文连续性

## 🏗️ 项目架构

```
ReEscape Protocol/
├── cmd/bot/                 # 程序入口
│   └── main.go             # 主程序，协程管理和信号处理
├── internal/
│   ├── config/             # 配置管理
│   │   └── config.go       # 环境变量加载和配置结构
│   ├── connect/            # WebSocket连接
│   │   └── connect.go      # WebSocket连接初始化
│   ├── state/              # 状态管理
│   │   └── manager.go      # 状态机和对话历史管理
│   ├── handler/            # 消息处理器
│   │   └── message.go      # 多层消息处理链
│   ├── scheduler/          # 自然定时器
│   │   └── natural.go      # 智能定时发送和消息池
│   ├── memory/             # 情感记忆
│   │   └── emotional.go    # 情感记忆和用户偏好分析
│   ├── character/          # 角色管理
│   │   └── character.go    # 角色配置加载和Prompt生成
│   ├── aifunction/         # AI功能集成
│   │   ├── aifunction.go   # AI接口封装
│   │   ├── base.go         # 基础AI功能
│   │   └── client.go       # AI客户端
│   ├── service/            # 业务服务
│   │   ├── group.go        # 群组服务
│   │   └── user.go         # 用户服务
│   ├── model/              # 数据模型
│   │   └── base.go         # 基础数据结构
│   └── utils/              # 工具函数
│       ├── logger.go       # 日志工具
│       └── retry.go        # 重试机制
├── config/character/        # 角色配置文件
│   └── default.json        # 默认角色配置
└── data/memory/            # 数据持久化
    ├── conversation_history.json  # 对话历史
    └── emotional_memory.json      # 情感记忆
```

## 🚀 安装指南

1. **环境要求**
   ```bash
   Go 1.23.6+
   ```

2. **克隆项目**
   ```bash
   git clone https://github.com/heky12356/ReEscape-Protocol.git
   cd ReEscape-Protocol
   ```

3. **安装依赖**
   ```bash
   go mod download
   ```

4. **配置环境**
   ```bash
   cp .env.example .env
   # 编辑 .env 文件，填入你的配置
   ```

## ⚙️ 配置说明

### 基础配置
- `HOSTADD`: OneBot WebSocket服务器地址 (如: localhost:8080)
- `TARGETID`: 目标用户QQ号
- `CHARACTER`: 角色配置文件名 (如: default)

### AI配置
- `AI_KEY`: AI服务API密钥
- `AI_BASEURL`: AI服务基础URL (如: https://api.openai.com/v1)
- `AI_MODEL`: 使用的AI模型 (如: gpt-3.5-turbo)
- `AI_PROMPT`: 额外的AI提示词
- `AI_TEMPERATURE`: AI创造性参数 (0.0-2.0，默认1.0)
- `AI_MAX_TOKENS`: 最大token数 (默认2000)
- `AI_TOP_P`: AI采样参数 (0.0-1.0，默认0.9)
- `AI_TIMEOUT`: AI请求超时时间(秒，默认30)

### 功能开关
- `ENABLE_NATURAL_SCHEDULER`: 启用自然定时器 (true/false)
- `ENABLE_EMOTIONAL_MEMORY`: 启用情感记忆 (true/false)
- `ENABLE_ONLY_LONG_CHAT`: 仅长对话模式 (true/false)

### 调度器配置
- `ACTIVE_HOURS`: 活跃时间段 (如: 9,10,11,14,15,16,19,20,21)
- `SLEEP_HOURS`: 休息时间段 (如: 0,1,2,3,4,5,6,7,8,22,23)
- `BASE_INTERVAL`: 基础发送间隔(分钟，默认45)
- `RANDOM_FACTOR`: 随机因子(0.0-1.0，默认0.5)

### 存储配置
- `DATA_DIR`: 数据目录 (默认./data)
- `LOG_DIR`: 日志目录 (默认./logs)

### 日志与观测配置
- `LOG_LEVEL`: 日志级别，支持 `DEBUG/INFO/WARN/ERROR`
- `LOG_TO_FILE`: 是否写入日志文件
- `LOG_FORMAT`: 日志格式，支持 `text/json`
- `LOG_ENABLE_COLOR`: 是否启用控制台彩色日志
- `REQUEST_ID_HEADER`: HTTP 请求 ID Header 名，默认 `X-Request-ID`
- `ENABLE_HEALTH_ENDPOINT`: 是否启用 `/healthz` 和 `/readyz`
- `ENABLE_METRICS`: 是否启用指标接口
- `METRICS_PATH`: 指标接口路径，默认 `/metrics`


## 🎮 快速开始

1. **启动程序**
   ```bash
   go run cmd/bot/main.go
   ```

2. **编译运行**（可选）
   ```bash
   go build -o bot cmd/bot/main.go
   ./bot
   ```

3. **使用启动脚本**
   ```bash
   # Windows
   start.bat
   
   # Linux/macOS
   ./start.sh
   ```

4. **创建必要目录**
   ```bash
   mkdir -p data
   mkdir -p data/memory
   mkdir -p logs
   ```

## 🔎 健康检查与指标

管理 HTTP 服务默认会暴露 3 个观测接口：

- `GET /healthz`: 存活检查。只回答“进程和 HTTP 服务是否还活着”。
- `GET /readyz`: 就绪检查。回答“服务现在是否适合接流量”。
- `GET /metrics`: 指标出口。给 Prometheus 一类监控系统抓取运行指标。

### `GET /healthz`

用途：
- 适合做 `liveness probe`
- 进程还活着时返回 `200 OK`

返回示例：

```json
{
  "status": "ok",
  "time": "2026-06-21T12:00:00+08:00",
  "uptimeSec": 3600,
  "checks": {
    "process": "ok"
  }
}
```

### `GET /readyz`

用途：
- 适合做 `readiness probe`
- 当前会检查配置是否可用、`DATA_DIR` 是否可写、`LOG_DIR` 是否可写
- 如果依赖异常，返回 `503 Service Unavailable`

返回示例：

```json
{
  "status": "ok",
  "time": "2026-06-21T12:00:00+08:00",
  "uptimeSec": 3600,
  "checks": {
    "config": "ok",
    "data_dir": "ok",
    "log_dir": "ok"
  }
}
```

异常时示例：

```json
{
  "status": "degraded",
  "time": "2026-06-21T12:00:00+08:00",
  "uptimeSec": 3600,
  "checks": {
    "config": "ok",
    "data_dir": "CreateFile ./data/.health-xxx: access is denied",
    "log_dir": "ok"
  }
}
```

### `GET /metrics`

用途：
- 暴露基础运行指标
- 适合 Prometheus 定时抓取
- 指标路径可通过 `METRICS_PATH` 调整

当前已包含的指标类别：
- HTTP 请求次数与耗时
- WebSocket 消息接收、丢弃、处理失败、处理成功次数
- 入站 pipeline 各阶段通过、丢弃、错误次数
- AI 分类和对话请求次数与耗时
- 持久化 flush 成功/失败次数
- 进程运行时长

返回示例：

```text
# HELP bot_uptime_seconds Process uptime in seconds.
# TYPE bot_uptime_seconds gauge
bot_uptime_seconds 3600
# HELP bot_http_requests_total Total HTTP requests by path, method, and status.
# TYPE bot_http_requests_total counter
bot_http_requests_total{method="GET",path="/healthz",status="200"} 12.000000
```

## 📖 详细文档

- **[使用指南](./使用指南.md)** - 详细的安装、配置和使用说明
- **[角色配置指南](./config/character/)** - 角色系统配置说明
- **[API文档](#)** - 接口和模块文档 (开发中)

## 🎯 使用场景

- **个人助手**: 智能对话，情感陪伴
- **客服机器人**: 自动回复，问题解答
- **学习伙伴**: 知识问答，学习辅导
- **娱乐聊天**: 角色扮演，趣味对话

## 📋 开发计划

### 🔥 近期计划 (v1.1)
- [ ] **Web管理界面**: 可视化配置和实时监控
- [ ] **多媒体支持**: 发送表情包、图片、语音消息
- [ ] **插件系统**: 支持第三方功能扩展
- [ ] **数据库支持**: MySQL/PostgreSQL 数据持久化

### 🚀 中期计划 (v1.2)
- [ ] **群聊支持**: 扩展到群聊场景，支持多人对话
- [ ] **情感分析可视化**: 图表展示情感变化趋势
- [ ] **多用户支持**: 同时服务多个用户
- [ ] **API接口**: RESTful API 和 WebHook 支持

### 🌟 长期计划 (v2.0)
- [ ] **健全人格管理**: 完善的人格配置和行为模拟
- [ ] **机器学习集成**: 个性化学习和适应
- [ ] **语音交互**: 语音识别和语音合成
- [ ] **移动端应用**: iOS/Android 客户端

### 🛠️ 技术改进
- [ ] **性能优化**: 内存使用和响应速度优化
- [ ] **安全增强**: 数据加密和访问控制
- [ ] **容器化部署**: Docker 和 Kubernetes 支持
- [ ] **监控告警**: 系统监控和故障告警

## 🤝 贡献指南

欢迎提交Issue和Pull Request来改进项目！

## 📄 许可证

本项目采用MIT许可证
