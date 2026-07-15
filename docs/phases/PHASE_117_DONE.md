# Phase 117 完成 — 群聊已读人数

## 需求分析

- 现状：`read-state` 返回全员，但 UI 只取任意一个 peer，群聊双勾无效。
- 目标：群消息显示「已读 N」，点击查看已读成员列表。

## 设计计划

- `memberReadState` 全量保存；WS `read` 增量更新
- `groupReadCount` / `groupReadMembers` / `groupPeerCount`
- ChatView 群消息已读按钮 + 浮层

## 交付

- chat store loadReadState / helpers / onFrame
- ChatView group-read UI

## 测试

- `npm run build` + `go build ./...`

## 下阶段

Phase 118：语音消息（录制 / 发送 / 播放）
