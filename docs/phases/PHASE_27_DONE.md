# Phase 27 完成

## 需求分析
- 好友仅有昵称/用户名，无法设置私人备注
- 聊天图片无法大图预览
- WebSocket 断开时消息静默丢失，无法重发

## 设计计划
- DB：`friendships.remark` 字段
- API：`PUT /friends/:id/remark`；好友列表返回 `remark`
- 前端：资料页编辑备注；`friendDisplayName` 优先备注
- `ImageLightbox` 组件点击放大
- 消息 `status`：sending/sent/failed + `retryMessage`

## 交付
- `deploy/init/mysql/003_friend_remark.sql`
- `backend/internal/store/friend.go`、`service/friend.go`、`handler/friend.go`
- `frontend/src/components/ImageLightbox.vue`
- `frontend/src/stores/chat.ts`、`views/ProfileView.vue`、`views/ChatView.vue`

## 测试
```powershell
go build ./...
npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 好友资料页可设备注，侧栏与聊天标题显示备注名
- 点击图片全屏预览
- 断网发送失败后显示「点击重试」

## 下阶段
Phase 28：聊天框粘贴图片发送
