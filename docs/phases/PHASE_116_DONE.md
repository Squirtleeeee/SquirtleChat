# Phase 116 完成 — 链接预览卡片

## 需求分析

- 现状：消息内 URL 仅可点击。
- 目标：抓取 OG 元数据展示标题/摘要/图；防 SSRF。

## 设计计划

- `GET /link-preview?url=` + 内存缓存 30m
- 禁止 localhost/私网；超时 6s；体限制 512KB
- `LinkPreviewCard` 挂在文本气泡下

## 交付

- `backend/internal/service/linkpreview/`
- `handler/link_preview.go` + bootstrap
- `frontend/src/components/LinkPreviewCard.vue` + ChatView

## 测试

- `go build ./...` + `npm run build` + 后端重启 ✅

## 下阶段

Phase 117：群聊已读人数 UI
