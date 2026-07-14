# Phase 91 完成

## 需求分析

用户希望有「智能体」陪聊，像微信里固定一个 AI 好友，用于日常聊天、答疑、给建议。

现状：仅有真人好友单聊，无 bot 账号与 LLM 接入。

验收：
- 登录后好友列表出现「杰尼助手」（`squirtle_ai`）
- 向助手发文本，数秒内收到回复（有 `LLM_API_KEY` 时为智能回复，否则为配置提示）
- 回复走现有 WS 消息流，多端可见
- `go build`、`npm run build`、冒烟含 agent 检查

## 设计计划

1. **Bot 用户**：启动时确保 `squirtle_ai` /「杰尼助手」存在
2. **自动好友**：注册/登录 + `POST /agent/ensure` 双向加好友
3. **AgentService**：监听用户→bot 文本消息 → OpenAI 兼容 API → `InjectAgentReply`
4. **配置**：`LLM_API_BASE`、`LLM_API_KEY`、`LLM_MODEL`
5. **前端**：`ensureAgent`、侧栏 AI 徽章、助手置顶排序

## 交付

### 后端
- `backend/internal/agent/llm.go` — OpenAI 兼容客户端
- `backend/internal/service/agent.go` — bot 生命周期、回复、typing
- `backend/internal/service/message.go` — `InjectAgentReply`
- `backend/internal/handler/agent.go` — `GET/POST /agent/*`
- `backend/internal/store/friend.go` — `EnsureFriendship`
- `backend/pkg/config/config.go` — LLM 环境变量
- `gateway-ws` — Kafka/本地 onSent 触发 agent

### 前端
- `frontend/src/constants/agent.ts`
- `frontend/src/stores/chat.ts` — `ensureAgent`、助手置顶
- `frontend/src/views/ChatView.vue` — AI 徽章样式

## 测试

```powershell
cd backend; go build ./...
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

通过（agent info 返回 `squirtle_ai`）

## 验证

- [x] 注册/登录自动加助手好友
- [x] `/agent/info` 返回 bot 资料
- [x] 无 API Key 时返回友好配置提示
- [x] 侧栏显示 AI 徽章并置顶

### 启用智能对话

启动 gateway 前设置环境变量（示例 OpenAI）：

```powershell
$env:LLM_API_KEY = "sk-..."
$env:LLM_API_BASE = "https://api.openai.com/v1"   # 可选，默认即此
$env:LLM_MODEL = "gpt-4o-mini"                      # 可选
.\scripts\start-backend.ps1
```

DeepSeek 等兼容接口可改 `LLM_API_BASE` 为对应地址。

## 下阶段

Phase 92：助手欢迎语、流式回复、或群聊 @助手（喊停即停）
