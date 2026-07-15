# Phase 145 完成 — 群成员备注（仅自己可见）

## 需求分析

- 群名片（141）全员可见；缺「仅自己可见」的成员备注（对标微信备注）。
- 验收：可对其他成员设/清备注；仅自己 GetGroup 可见；会话显示优先备注。

## 设计计划

- 表 `group_member_remarks`（025）
- `PUT /groups/:id/members/:uid/remark`
- GetGroup 返回 `member_remarks`
- FE：成员列表「备注」弹层；显示优先级 备注 > 群名片 > 好友备注 > 昵称

## 交付

- Migration + store/service/handler
- chat.setGroupMemberRemark / groupMemberRemarks
- GroupDetailView 备注 UI

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- API smoke：设备注后 GetGroup 返回 member_remarks

## 下阶段

Phase 146：消息翻译按钮（调现有 LLM）或已读回执详情增强
