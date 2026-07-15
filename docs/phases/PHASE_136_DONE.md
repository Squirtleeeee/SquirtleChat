# Phase 136 完成 — 链接预览失败重试

## 需求分析

- OG 卡片失败时此前静默消失；应对标主流 IM：展示失败态并允许刷新缓存重试。
- 验收：失败显示站点 +「重试」；`refresh=1` 绕过成功/失败缓存。

## 设计计划

- 后端：失败短缓存 2min；`FetchOpt(..., refresh)` 可清缓存重抓
- 前端：`LinkPreviewCard` 失败态 + 重试按钮

## 交付

- `linkpreview.FetchOpt` + handler `?refresh=1`
- `LinkPreviewCard.vue` 失败 UI / retry

## 测试

- `go build ./...` ✅
- `npm run build` ✅

## 下阶段

Phase 137：草稿冲突提示（本地 vs 云端草稿不一致时可选保留）
