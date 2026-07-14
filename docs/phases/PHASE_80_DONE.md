# Phase 80 完成

## 需求分析

对照 ui-ux-pro-max 与主流 IM（微信 / WhatsApp / Telegram）：
- 功能性图标不得用 emoji；侧栏「免打扰」「置顶」此前仍为 🔕 / 📌
- 好友列表与群列表需一致的矢量图标与触控尺寸

验收：侧栏静音/置顶均为 SVG；`npm run build` 通过。

## 设计计划

- 好友行 / 群行 mute、pin 替换为 stroke/fill SVG
- `.mute-dot` / `.pin-btn` 改为 inline-flex，置顶态琥珀色
- 无后端/协议变更

## 交付

- `frontend/src/views/ChatView.vue` — 侧栏静音铃铛划线 + 置顶图钉 SVG 与样式

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 免打扰显示铃铛划线图标；置顶图钉在 pinned 态高亮
- 好友与群列表图标一致

## 下阶段

Phase 81：粘性日期分隔 + 文件气泡 SVG（对齐 WhatsApp/微信）
