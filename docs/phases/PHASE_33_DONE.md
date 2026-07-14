# Phase 33 完成

## 需求分析
- WS 断开后固定 1.5s 重连，无退避，弱网易打爆服务
- 状态仅显示「未连接 / 连接中」，无横幅与手动重连
- 重连成功后不立刻 `pullSync`，离线消息依赖 3s 轮询才补齐
- `reconnectWS()` 新建 `WSClient` 后 `bindWS` 因 `wsBound` 跳过，帧/状态 handler 丢失

## 设计计划
- `WSClient`：指数退避（1.5s → 30s）、`forceReconnect`、安全 `detachSocket`
- `auth.reconnectWS`：复用现有 client 强制重连，避免换实例丢 handler
- `chat.bindWS`：始终重新挂载；断线标记 `wasDisconnected`；重连后立刻 `pullSync`
- UI：断线/重连横幅 +「立即重连」；状态文案显示重连次数 / 同步中

## 交付
- `frontend/src/api/ws.ts`：退避重连、`forceReconnect`、`reconnectAttempt`
- `frontend/src/stores/auth.ts`：`reconnectWS` 走 `forceReconnect`
- `frontend/src/stores/chat.ts`：重连后同步、`forceReconnect`、短暂成功提示
- `frontend/src/views/ChatView.vue`：`reconnect-banner`、状态文案

## 测试
```powershell
cd frontend; npm run build
cd backend; go build ./...
```

## 验证
- 断开网关后出现黄色横幅，可点「立即重连」
- 自动重连间隔逐步变长（约 1.5s / 3s / 6s … 封顶 30s）
- 重连成功后提示「连接已恢复，消息已同步」，离线期间消息尽快出现
- 发送失败触发重连后仍能收到后续 WS 消息（handler 未丢失）

## 下阶段
Phase 34：消息搜索（会话内关键词）/ 跳转到历史位置
