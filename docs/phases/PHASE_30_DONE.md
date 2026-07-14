# Phase 30 完成

## 需求分析
- 用户发错消息需要在 2 分钟内撤回
- 对方会话内应实时看到「[已撤回]」

## 设计计划
- `POST /conversations/:id/messages/:msg_id/recall`：仅发送者、2 分钟内、未撤回
- DB：`UPDATE messages SET msg_type=4, content='[已撤回]'`
- WS 广播 `recall` 帧给会话成员
- 前端：自己消息悬停「撤回」、处理 `recall` 帧

## 交付
- `backend/internal/store/message.go`：`GetByID`、`Recall`、`RecallEvent`
- `backend/internal/service/message.go`：`RecallMessage`、`SetOnRecalled`
- `backend/internal/handler/message.go`：recall 路由
- `backend/internal/push/dispatcher.go`：`BroadcastRecall`
- `backend/services/gateway-http/main.go`：HTTP 撤回时推送 WS
- `frontend/src/stores/chat.ts`：`recallMessage`、处理 `recall` 帧
- `frontend/src/views/ChatView.vue`：撤回按钮、已撤回样式
- `frontend/src/utils/format.ts`：预览 `[已撤回]`

## 测试
```powershell
cd backend; go build ./...
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 发送文本后 2 分钟内点「撤回」，气泡变为「[已撤回]」
- 对方窗口同步更新（需 WS 在线）
- 超过 2 分钟 API 返回错误

## 下阶段
Phase 31：README 完善 + smoke 测试覆盖好友备注
