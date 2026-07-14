# Phase 92 完成

## 需求分析

1. **杰尼助手无法智能回复**：用户配置 DeepSeek API 后仍收到「抱歉，我暂时无法回复」。
2. **未读红点逻辑错误**：用户在自己当前会话发消息时，左侧好友列表出现未读红点；正确行为应仅在**他人发来未读消息**时提示。

## 设计计划

- **LLM**：在 Go `config.Load()` 自动读取 `deploy/llm.env`，避免仅 HTTP 网关加载环境变量而 WS 网关仍用默认 `api.openai.com` 导致 DeepSeek Key 鉴权失败。
- **未读（后端）**：发送消息成功后，将发送方 `last_read_seq` 推进到当前 `seq`，避免 `last_seq - last_read_seq` 把己方消息算作未读。
- **未读（前端）**：当前打开会话不显示未读；ACK 后本地同步 `last_read_seq`；顶部 Tab 未读数排除当前会话。

## 交付

| 文件 | 变更 |
|------|------|
| `backend/pkg/config/config.go` | 启动时加载 `deploy/llm.env` |
| `backend/internal/store/message.go` | `BumpLastReadSeq` |
| `backend/internal/service/message.go` | 发送后更新发送方已读游标 |
| `frontend/src/stores/chat.ts` | 未读展示与 ACK 本地同步 |
| `scripts/start-backend.ps1` | WS 启动时打印 LLM 配置状态 |

## 测试

```powershell
cd backend; go build ./...
cd frontend; npm run build
```

## 验证

- [ ] 重启 `.\scripts\start-backend.ps1`，WS 窗口显示 `LLM: configured (https://api.deepseek.com/v1)`
- [ ] 向杰尼助手发消息，应收到 DeepSeek 智能回复（非配置提示/无法回复）
- [ ] 在当前会话发消息，左侧列表不出现未读红点/数字

## 下阶段

Phase 93：杰尼助手欢迎语、流式回复或群聊 @助手。
