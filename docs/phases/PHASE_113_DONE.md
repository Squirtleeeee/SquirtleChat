# Phase 113 完成 — 好友在线状态

## 需求分析

- Redis `online:{uid}:{device}` 已有；UI 无好友在线点。
- 目标：批量查询在线并在好友列表展示绿点；定时刷新。

## 设计计划

- `Router.BatchOnline` / `IsUserOnline`
- `POST /users/presence` `{ user_ids }`
- FE `refreshPresence` + 列表绿点 + 20s 轮询

## 交付

- routing/router.go、auth handler、bootstrap
- chat store + ChatView online-dot

## 测试

- `go build ./...` + `npm run build` + 后端重启 ✅

## 下阶段

Phase 114：全局消息搜索
