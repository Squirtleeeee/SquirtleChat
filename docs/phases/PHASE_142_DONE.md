# Phase 142 完成 — 群邀请链接（可撤销 / 限次）

## 需求分析

- 现状：入群依赖好友邀请、群号申请、面对面码；缺少可分享的邀请链接。
- 对标：Telegram / Discord invite links。
- 验收：管理员可生成 8 位邀请码（可选次数上限与过期）；可撤销；对方预览后直接入群。

## 设计计划

- 表 `group_invite_links`（022）
- API：创建/列表/撤销；预览；加入（原子消耗次数）
- FE：群详情管理区；添加联系人「邀请码」页签

## 交付

- `deploy/init/mysql/022_group_invite_links.sql`
- store/service/handler invite-link CRUD + join
- `chat.createGroupInviteLink` / `joinViaInviteLink` 等
- `GroupDetailView` 邀请链接区；`AddContactModal` 邀请码 tab

## 测试

- Migration 已应用
- `go build ./...` ✅
- `npm run build` ✅
- API smoke：创建群 → 生成链接 → test_b 预览/加入

## 下阶段

Phase 143：群慢速模式（发言冷却）或会话免打扰时段
