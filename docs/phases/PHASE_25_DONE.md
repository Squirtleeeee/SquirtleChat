# Phase 25 完成

## 需求分析
- 会话列表缺少微信式相对时间（刚刚、5分钟前）
- 单聊自己发送的消息无已读回执
- 收到新消息时页面在后台无提醒
- 对方已读时发送方无法实时感知

## 设计计划
- `formatListTime` 用于侧栏会话时间
- 后端：`MarkRead` 后 WS 推送 `read` 帧；`GET /conversations/:id/read-state` 查询成员已读位置
- 前端：`peerReadSeq` 状态、气泡「已读」标签
- `utils/notify.ts`：浏览器 Notification（页面隐藏或非当前会话时）

## 交付
- `backend/internal/store/message.go`：成员已读查询
- `backend/internal/service/sync.go`：`GetReadState`、`SetOnRead`
- `backend/internal/push/dispatcher.go`：`BroadcastRead`
- `backend/internal/handler/sync.go`：`read-state` 端点
- `frontend/src/utils/format.ts`：`formatListTime`
- `frontend/src/utils/notify.ts`
- `frontend/src/stores/chat.ts`：已读 WS、桌面通知
- `frontend/src/views/ChatView.vue`：列表时间、已读标签

## 测试
```powershell
go build ./...
npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 侧栏好友行右侧显示相对时间
- A 发消息，B 打开会话后 A 看到自己消息标「已读」
- 标签页在后台时收到消息弹出系统通知（需授权）

## 下阶段
Phase 26：聊天日期分割线 + 侧栏会话搜索
