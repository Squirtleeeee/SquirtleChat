# Phase 19 完成

## 需求分析
- 编辑资料仅支持昵称，缺少仿微信独立资料页
- 需支持头像上传与圆形裁剪、性别、生日
- 需隐私设置且后端必须强制执行（他人不可见时仅保留用户名）

## 设计计划
- DB：`gender`、`birthday`、`privacy_json` 字段（`002_profile_privacy.sql`）
- API：`PUT /users/me`（部分更新）、`PUT /users/me/privacy`、`POST /users/me/avatar`、`GET /users/:id`（隐私过滤）
- 前端：`ProfileView`、`EditProfileView`、`AvatarCropper` 组件

## 交付
- `backend/internal/model/model.go`：`UserPrivacy`、`PublicProfile`、`ApplyPrivacy`
- `backend/internal/store/user.go`：扩展资料读写
- `backend/internal/handler/auth.go`：新端点
- `frontend/src/views/ProfileView.vue`、`EditProfileView.vue`
- `frontend/src/components/AvatarCropper.vue`

## 测试
```powershell
Get-Content deploy\init\mysql\002_profile_privacy.sql -Raw | mysql -u squirtle -psquirtle123 squirtlechat
go build ./...
npm run build
```

## 验证
- 关闭全部隐私后，`GET /users/:id` 仅返回 `id` + `username`
- 头像裁剪上传后 `/users/me` 返回新 `avatar` URL

## 下阶段
Phase 20：主界面简化与添加好友弹窗
