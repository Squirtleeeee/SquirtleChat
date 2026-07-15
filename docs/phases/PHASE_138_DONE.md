# Phase 138 完成 — 会话搜索结果上下导航

## 需求分析

- 对标浏览器/IDE 查找：在结果间「上一条 / 下一条」跳转并定位气泡。
- 验收：搜索面板导航条；F3 / Shift+F3 / Ctrl+G；输入框 ↑↓；当前命中高亮。

## 交付

- `searchHitIndex` + `jumpSearchHit`
- 搜索面板导航 UI + 快捷键
- 列表项 `.active` 高亮

## 测试

- `npm run build` ✅
- `go build ./...` ✅

## 下阶段

Phase 139：消息多选批量收藏 / 批量转发体验收尾
