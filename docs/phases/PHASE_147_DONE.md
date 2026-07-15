# Phase 147 完成 — 群欢迎语 / 入群系统提示

## 需求分析

- 入群无系统提示与欢迎语；对标微信「xxx加入了群聊」+ 自定义欢迎。
- 验收：管理员设欢迎语；邀请接受/邀请链接/面对面入群后发系统消息。

## 交付

- `groups.welcome_text`（026）
- `PUT /groups/:id/welcome`
- `PostSystemMessage`；`announceMemberJoined`
- GroupDetailView 入群欢迎语编辑

## 测试

- `go build ./...` ✅
- `npm run build` ✅

## 下阶段

Phase 148：会话内群公告横幅（可关闭）
