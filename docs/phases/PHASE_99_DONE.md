# Phase 99 完成 — 群管理员与建群码管理

## 需求分析

- 群主可设置/取消管理员
- 管理员与群主均可邀请好友、刷新建群码
- 群详情展示成员角色与有效面对面建群码

## 交付

- `POST/DELETE /groups/:id/admins/:uid` — 设置/取消管理员
- `POST /groups/:id/face-to-face/refresh` — 刷新建群码
- `GET /groups/:id/face-to-face` — 查看当前建群码
- `GET /groups/:id` 响应增加 `member_roles`
- `GroupDetailView` — 管理员标签、设管理员、建群码区块

## 测试

`go build ./...`、`npm run build`、冒烟测试通过。

## 下阶段

Phase 100：群主查看待处理入群邀请、踢人、转让群主
