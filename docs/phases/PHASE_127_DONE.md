# Phase 127 完成 — 导出会话记录

## 需求分析

- 合规/备份：将会话导出为纯文本。
- 验收：成员可下载最多约 2000 条记录的 `.txt`。

## 交付

- `GET /conversations/:id/export`（text/plain）
- ChatView「导出」按钮

## 测试

builds ✅ · backend 重启 ✅

## 下阶段

Phase 128：投票消息（Poll）或未读 @ 提及聚合
