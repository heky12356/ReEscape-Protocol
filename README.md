<div align="center">

# ReEscape Protocol

</div>

---

## 功能特性
- 定时消息发送：系统会按照设定的时间间隔发送预设消息。
- 条件回复机制：根据不同的回复内容返回不同的消息。
- AI集成：计划接入AI模型并支持长对话。
  支持自定义配置baseurl, model_name, api_key以及prompt。


## 安装指南
1. 确保已安装Go语言环境（推荐版本1.23.6）
2. 克隆本仓库到本地
3. 进入项目目录，执行`go mod download`命令安装依赖

## 使用说明
1. 配置环境变量：创建`.env`文件，并设置以下参数：
   - HOSTADD: WebSocket服务器地址
   - GROUPID: 默认群组ID
   - TARGETID: 目标用户ID
   - AI_KEY: AI服务密钥
   - AI_BASEURL: AI服务基础URL
   - AI_PROMPT: AI提示词模板
   - AI_MODEL: 使用的AI模型名称
2. 执行`go build -o bot cmd/bot/main.go`命令编译程序
3. 创建`public/aichatlog`目录和`public/aichatlog/longchain`，用于对话日志
4. 运行生成的可执行文件`bot`
> 或许直接在根目录下执行`go run cmd/bot/main.go`会更好些


## 计划
- 动态定时器：根据收到的回复调整下一次发送的时间间隔。
- 模拟真人互动：未来计划使用随机定时器并延长间隔，以模拟自然消息发送行为。
- 多媒体支持：后续计划支持发送表情包和“戳一戳”功能。