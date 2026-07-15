# Phase 129 完成 — 投票消息（Poll）

## 需求分析

- 对标 Slack / 企微投票：会话内发起单选投票，实时汇总票数。
- 验收：composer「投票」；气泡展示选项/占比；投票后 WS 同步；历史带 `polls`。

## 设计计划

- `msg_type=6` + content `{question, options[{id,text}]}`
- 表 `poll_votes` PK `(msg_id, user_id)` 单选可改投
- `POST .../messages/:msg_id/poll/vote` + WS `poll_vote`
- 列表消息附带 `polls` 结果

## 交付

- Migration `016_poll_votes.sql`
- Store/Service/Handler + `BroadcastPollVote`（HTTP/WS）
- ChatView 投票气泡、composer、样式；chat store `sendPoll` / `votePoll`

## 测试

- `go build ./...` ✅
- `npm run build` ✅（修复未使用 `opt` 绑定）

## 下阶段

Phase 130：群公告置顶（Group announcement）
