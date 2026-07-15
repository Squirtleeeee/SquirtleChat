# Phase 140 完成 — 群成员禁言 / 仅管理员发言

## 需求分析

- 现状：群管理可踢人/设管理员/公告，但无法限制发言。
- 对标：微信/钉钉「全员禁言」「单人禁言」。
- 验收：管理员开关全员禁言；对成员禁言/解禁；被禁用户发送失败且输入框禁用；群主/管理员仍可发言。

## 设计计划

- Migration `020_group_mute.sql`：`groups.admin_only`、`group_members.muted`
- API：`PUT /groups/:id/admin-only`；`POST|DELETE /groups/:id/members/:uid/mute`
- 发送前 `MessageService.ensureCanPost`
- FE：群详情发言管理 + 成员禁言按钮；会话输入框按权限禁用

## 交付

- `deploy/init/mysql/020_group_mute.sql`
- Backend store/service/handler mute + admin-only
- `chat.setGroupAdminOnly` / `setGroupMemberMuted` / `canPostInActiveGroup`
- `GroupDetailView` 发言管理；`ChatView` composer 拦截

## 测试

- Migration 已应用到本地 MySQL
- `go build ./...` ✅
- `npm run build` ✅

## 下阶段

Phase 141：群内成员昵称（群名片）或群邀请链接
