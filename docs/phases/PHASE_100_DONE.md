# Phase 100 完成 — 踢人、转让群主、待处理邀请

## 需求分析

- 群主/管理员可移出成员（管理员仅能移出普通成员）
- 群主可转让群主
- 群主须先转让后才能退出
- 群主/管理员可查看并撤销群内待处理入群邀请

## 交付

### 后端

- `KickGroupMember` — 踢人 / 自己退群
- `TransferGroupOwner` — 转让群主并更新角色
- `ListGroupPendingInvites` / `CancelGroupInvite`
- `GET /groups/:id/invitations`、`DELETE /groups/:id/invitations/:inviteId`
- `POST /groups/:id/transfer`、`DELETE /groups/:id/members/:uid`（支持踢人）

### 前端

- `GroupDetailView` — 待处理邀请、移出成员、转让群主、退出群聊
- `chat.ts` — 对应 API 封装

## 测试

`go build ./...`、`npm run build`、冒烟测试扩展（踢人、待处理邀请列表）

## 下阶段

Phase 101：群名称修改、解散群聊、邀请消息 WS 推送
