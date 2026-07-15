# Phase 107 完成 — 修改密码

## 需求分析

- 现状：仅有注册/登录，无法在登录态修改密码（商用账号安全基线缺口）。
- 目标：已登录用户可在设置页用原密码验证后更新密码。
- 验收：`PUT /users/me/password`；错误原密码失败；新密码 <6 拒绝；设置页可成功改密。
- 本阶段不做：强制踢下线其它设备（→108）、找回密码邮件。

## 设计计划

- Store：`UpdatePasswordHash`
- Service：`ChangePassword`（bcrypt 校验 + 更新）
- Handler：`PUT /users/me/password` `{ old_password, new_password }`
- FE：`auth.changePassword` + 设置页「账号安全」表单

## 交付

- `backend/internal/store/user.go` — `UpdatePasswordHash`
- `backend/internal/service/auth.go` — `ChangePassword`
- `backend/internal/handler/auth.go` — route + handler
- `backend/internal/service/auth_test.go` — 校验单测
- `frontend/src/stores/auth.ts` — `changePassword`
- `frontend/src/views/SettingsView.vue` — 改密表单

## 测试

- `go test ./internal/service/ -count=1`（含 ChangePasswordValidation）
- `go build ./...` + `npm run build`

## 验证

- [ ] 设置 → 账号安全 → 错误原密码提示
- [ ] 正确原密码 + 新密码 ≥6 → 成功提示
- [ ] 新密码可登录

## 下阶段

Phase 108：登录设备列表 + 远程下线（`user_devices`）
