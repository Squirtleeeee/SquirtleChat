# Phase 143 完成 — 群慢速模式

## 需求分析

- 对标 Discord Slowmode：限制普通成员发言频率，抑制刷屏。
- 验收：管理员设置 0–3600 秒间隔；普通成员冷却期内发送失败；管理员/群主不受限；输入区展示原因。

## 设计计划

- `groups.slow_mode_secs`（023）
- `PUT /groups/:id/slow-mode`
- `ensureCanPost` 查最近一条本人消息时间
- FE：群详情设置；`canPostInActiveGroup` 冷却判断

## 交付

- Migration + store/service/handler
- Chat store getters + GroupDetailView 慢速设置

## 测试

- `go build ./...` ✅
- `npm run build` ✅
- Migration 已应用

## 下阶段

Phase 144：会话免打扰时段（DND schedule）或消息翻译入口
