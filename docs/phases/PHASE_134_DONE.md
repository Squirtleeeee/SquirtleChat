# Phase 134 完成 — 群聊已读详情增强

## 需求分析

- 对标微信群已读：列表带头像，可点进资料；同时展示未读成员。
- 验收：点「已读 N」弹出头像行；点击打开 `/profile/:id`；打开时刷新 read-state。

## 交付

- `groupUnreadMembers` store helper
- ChatView 已读/未读分区 + UserAvatar 可点
- 打开弹层时 `loadReadState`

## 测试

- `npm run build` ✅
- `go build ./...` ✅

## 下阶段

Phase 135：会话内「仅未读」快捷筛选 / 跳转到第一条未读
