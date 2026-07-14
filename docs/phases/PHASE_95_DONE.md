# Phase 95 完成

## 需求分析

杰尼助手持续返回「抱歉，我暂时无法回复」，但 `/agent/info` 显示 `llm: true`。

根因（经 E2E 诊断 `cmd/agentdiag`）：
- **运行中的 `gateway-ws` 进程** 只加载了 `LLM_API_KEY`，`LLM_API_BASE` 仍为默认 `api.openai.com`，DeepSeek Key 在 OpenAI 端点鉴权失败。
- `deploy/llm.env` 此前使用 `only-if-empty` 加载，无法覆盖进程里已有的错误环境变量。
- DeepSeek 官方 base 为 `https://api.deepseek.com`（非 `/v1`），`deepseek-chat` 已临近弃用，推荐 `deepseek-v4-flash`。

参考：[DeepSeek API 文档](https://api-docs.deepseek.com/)

## 设计计划

1. `llm.env` **强制覆盖** 所有 `LLM_*` 环境变量。
2. `normalizeLLMConfig`：DeepSeek Key/模型时自动纠正 OpenAI 默认 base。
3. LLM Client 统一去掉 base 末尾 `/v1`，请求 `/chat/completions`。
4. Agent 历史上下文跳过错误兜底文案，避免污染多轮对话。
5. `/agent/info` 返回 `llm_base` / `llm_model` 便于自检。
6. 重启 gateway 使配置生效。

## 交付

| 文件 | 变更 |
|------|------|
| `backend/pkg/config/config.go` | 强制加载 llm.env + DeepSeek 归一化 |
| `backend/internal/agent/llm.go` | base 规范化、错误详情 |
| `backend/internal/service/agent.go` | 过滤兜底历史、暴露 base/model |
| `backend/internal/handler/agent.go` | info 返回 llm_base/model |
| `deploy/llm.env` / `.example` | 官方 DeepSeek 配置 |
| `backend/cmd/agentdiag` | WS 端到端诊断工具 |

## 测试

```powershell
cd backend; go build ./...
go run ./cmd/agentdiag   # 应收到杰尼助手智能回复
```

结果：live gateway-ws 返回正常中文回复，非兜底错误。

## 验证

- [ ] `.\scripts\start-backend.ps1` 后 WS 窗口有 `llm configured base=https://api.deepseek.com`
- [ ] `GET /api/v1/agent/info` 中 `llm_base` 为 `https://api.deepseek.com`
- [ ] 向杰尼助手发消息收到智能回复

## 下阶段

Phase 96：流式回复、欢迎语、群聊 @助手。
