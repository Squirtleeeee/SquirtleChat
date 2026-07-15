# Phase 141 完成 — 群内昵称（群名片）

## 需求分析

- 现状：群聊展示个人昵称/好友备注，无法设置仅本群可见的称呼。
- 对标：微信「我在本群的昵称」。
- 验收：成员可设置/清空群名片；群详情与消息发送者名优先显示群名片；@提及识别群名片。

## 设计计划

- Migration `021_group_nickname.sql`：`group_members.nickname`
- `PUT /groups/:id/my-nickname`
- GetGroup 返回 `member_nicknames`
- FE：群详情编辑；`groupMemberDisplayName` / `mentionName` / `messageMentionsMe`

## 交付

- SQL + store/service/handler
- `chat.setMyGroupNickname`、`groupMemberNicknames`
- `GroupDetailView`「我的群名片」；`ChatView` 群聊 sender 名

## 测试

- Migration 已应用（local MySQL）
- `go build ./...` ✅
- `npm run build` ✅
- API smoke：`PUT .../my-nickname` ✅

## 下阶段

Phase 142：群邀请链接（可撤销 / 限次）或慢速模式
