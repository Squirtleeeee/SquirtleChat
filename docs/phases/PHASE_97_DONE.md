# Phase 97 完成

## 需求分析

杰尼龟龟回复过于「人机/百科腔」，需要：
- 更和气、二次元萌点、会哄着用户聊天
- 避免客服腔与长篇说明书式回答

## 设计计划

- 独立 `agent.SystemPrompt` 人设文档（system prompt）
- LLM `temperature: 0.88` 提升口语与自然度
- 兜底/欢迎语改为角色口吻

## 交付

| 文件 | 变更 |
|------|------|
| `backend/internal/agent/persona.go` | 杰尼龟龟人设 prompt |
| `backend/internal/agent/llm.go` | temperature |
| `backend/internal/service/agent.go` | 引用 prompt、角色化兜底文案 |

## 测试

```powershell
cd backend; go build ./...
```

## 验证

- [ ] 与杰尼龟龟闲聊，语气可爱、口语化，非百科列表
- [ ] 情绪低落时先安慰再接话

## 下阶段

欢迎语主动推送、流式回复。
