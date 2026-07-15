# Phase 125 完成 — 自定义状态

## 需求分析

- 对标 Slack/企微状态签名：表情 + 短文，好友列表与资料页可见。
- 验收：编辑资料可设状态；好友列表显示；资料页显示。

## 交付

- `014_user_status.sql`（status_text / status_emoji）
- PUT /users/me 支持状态字段
- EditProfile / Profile / 好友列表 UI

## 测试

builds ✅ · backend 重启 ✅

## 下阶段

Phase 126：定时发送消息
