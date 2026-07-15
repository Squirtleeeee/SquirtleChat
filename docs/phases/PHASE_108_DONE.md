# Phase 108 完成 — 登录设备列表与远程下线

## 需求分析

- 现状：`user_devices` 已有，登录会 Upsert，但无列表/撤销 API 与 UI。
- 目标：设置页展示设备、标记本机、可下线其它端并通过 WS `kick` 踢线。
- 验收：`GET /users/me/devices`、`DELETE /users/me/devices/:id`；被踢端退出到登录页。

## 设计计划

- Store：`ListDevices` / `DeleteDevice`；Upsert 保留已有 device_name
- Service：`ListDevices` / `RevokeDevice` / `TouchDevice`；Login 接收 `device_name`
- Hub：`KickDevice` 推送 kick 并关闭连接
- FE：设置页设备列表；`X-Device-Id`；WS kick → logout

## 交付

- backend store/service/handler/ws/bootstrap/agentdiag
- frontend auth store、http 拦截器、SettingsView、chat.onFrame kick

## 测试

- `go build ./...` + `go test ./internal/service/` + `npm run build` ✅

## 下阶段

Phase 109：通知设置（开关 / 仅提到我 / 免打扰时段）
