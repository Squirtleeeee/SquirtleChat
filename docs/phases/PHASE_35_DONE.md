# Phase 35 完成

## 需求分析
- 对方输入时无实时提示，聊天反馈偏「死」
- 现有 WS 仅处理 `message` / `ping`，无轻量信令通道

## 设计计划
- WS 帧 `typing`：`{ conversation_id, typing }`
- Hub → MessageService.HandleTyping → Dispatcher.BroadcastTyping（不落库、不进 Kafka）
- 前端输入节流 1.5s 发送 typing；收到后 4s 自动清除；发消息时发 `typing:false`
- UI：composer 上方「对方正在输入…」/ 群聊显示昵称

## 交付
- `backend/internal/ws/hub.go`：处理 `typing` 帧
- `backend/internal/service/message.go`：`HandleTyping`、`TypingEvent`
- `backend/internal/push/dispatcher.go`：`BroadcastTyping`
- `backend/services/gateway-ws/main.go`：挂载 onTyping
- `frontend/src/api/ws.ts`：`sendTyping`
- `frontend/src/stores/chat.ts`：typing 状态与文案
- `frontend/src/views/ChatView.vue`：输入指示条

## 测试
```powershell
cd backend; go build ./...
cd frontend; npm run build
```

## 验证
- 双账号同会话：A 输入时 B 看到「对方正在输入…」
- 停止输入约 4s 后提示消失
- 发送消息后对方提示立即消失
- 群聊显示「昵称 正在输入…」

## 下阶段
Phase 36：消息草稿本地持久化 / 切换会话保留未发送内容
