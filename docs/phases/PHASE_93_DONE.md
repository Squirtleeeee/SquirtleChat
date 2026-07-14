# Phase 93 完成

## 需求分析

1. **杰尼助手仍返回「暂时无法回复」**：Phase 92 后问题依旧，需从消息派发链路排查。
2. **发消息后聊天区不跟随滚动**：用户发送或收到新消息时，消息列表应自动滚到最新位置。

## 设计计划

- **Agent 派发**：`HandleSend` 成功后**立即** `onSent`（本地 WS 推送 + 触发 Agent），不再依赖 Kafka 消费成功才跑 Agent；Kafka 仅负责跨实例推送，并带 `origin_instance` 去重。
- **LLM 配置**：`deploy/llm.env` 多路径绝对路径解析 + 启动日志。
- **自动滚动**：发送后强制滚底；己方消息与贴底状态下新消息自动滚底；`requestAnimationFrame` 等待布局后再滚动。

## 交付

| 文件 | 变更 |
|------|------|
| `backend/internal/service/message.go` | 始终 onSent；OriginInstance |
| `backend/internal/store/message.go` | SentEvent.OriginInstance |
| `backend/services/gateway-ws/main.go` | Kafka 去重；Agent 仅走 onSent |
| `backend/pkg/config/config.go` | llm.env 绝对路径加载 |
| `backend/internal/app/bootstrap.go` | LLM 启动日志 |
| `frontend/src/views/ChatView.vue` | 发消息/新消息自动滚底 |

## 测试

```powershell
cd backend; go build ./...
cd frontend; npm run build
```

## 验证

- [ ] 重启 `.\scripts\start-backend.ps1`，WS 窗口有 `llm configured base=https://api.deepseek.com/v1`
- [ ] 杰尼助手回复正常中文，非错误兜底
- [ ] 发消息后视图自动滚到最新消息
- [ ] 打开会话后显示历史最底部

## 下阶段

Phase 94：流式回复、助手欢迎语、群聊 @助手。
