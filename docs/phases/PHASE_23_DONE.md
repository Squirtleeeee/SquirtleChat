# Phase 23 完成

## 需求分析
- 好友/群列表未按最近聊天排序，无消息预览，不像微信会话列表
- 聊天气泡无时间显示
- 无法从聊天页进入对方资料/群信息
- 好友资料页缺少「发消息」入口
- 添加好友时无法填写验证消息
- 会话列表排序依赖 `conversations.updated_at`，发消息后未刷新

## 设计计划
- 后端：`ListConversations` 按最后消息时间排序，返回 `last_msg_type`；`Insert` 时更新会话 `updated_at`；`GET /groups/:id` 返回成员 `PublicProfile`
- 前端：`sortedFriends` / `sortedGroups` getter；`format.ts` 时间与预览；`GroupDetailView`；资料页发消息；添加好友验证消息

## 交付
- `backend/internal/store/message.go`：会话排序、last_msg_type、touch updated_at
- `backend/internal/service/friend.go`：群成员公开资料
- `frontend/src/utils/format.ts`
- `frontend/src/views/GroupDetailView.vue`
- `frontend/src/views/ChatView.vue`：预览、时间、标题跳转
- `frontend/src/views/ProfileView.vue`：发消息
- `frontend/src/components/AddContactModal.vue`：验证消息

## 测试
```powershell
go build ./...
npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 有聊天记录的好友排在列表前面，显示最后一条预览（文本/图片/文件）
- 点击聊天标题进入资料或群信息页
- 好友资料页可「发消息」跳回聊天

## 下阶段
Phase 24：WebSocket 真实连接状态 + 历史消息上拉加载
