# Phase 109 完成 — 消息通知设置

## 需求分析

- 现状：仅有浏览器 Notification 权限请求；无全局开关与免打扰时段。
- 目标：设置页可开关桌面通知、申请权限、配置静默时段；会话 mute 仍优先。
- 验收：关闭全局通知后不再弹桌面 toast；静默时段内不弹。

## 设计计划

- `settings` store：`notify` prefs + `shouldDesktopNotify()`
- `chat.onFrame` 调用 settings 门禁
- SettingsView「消息通知」区块

## 交付

- `frontend/src/stores/settings.ts`
- `frontend/src/stores/chat.ts`
- `frontend/src/views/SettingsView.vue`

## 测试

- `npm run build` + `go build ./...`

## 下阶段

Phase 110：免打扰 / 置顶偏好服务端同步（多端一致）
