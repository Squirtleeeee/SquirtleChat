# Phase 90 完成

## 需求分析

现象：聊天中无法正常上传/查看文件（图片、附件）。

现状：
- `POST /api/v1/files/upload` 在 MinIO 可用时会把对象写入 MinIO
- 返回 URL 为 `http://localhost:9000/squirtlechat/{key}`，桶默认私有 → 浏览器 **403**
- 启用 MinIO 后 `bootstrap` 不再挂载 `/uploads` 静态目录
- 前端 axios 手动设置 `Content-Type: multipart/form-data` 可能缺少 boundary，导致部分环境解析失败

验收：
- 上传返回 `/uploads/{key}`
- `GET http://localhost:8080/uploads/{key}` 可下载/预览
- 历史 MinIO 直链消息在前端可正常展示
- `go build ./...`、`npm run build`、冒烟含文件上传

## 设计计划

1. **后端** `FileService.Upload` 统一返回 `/uploads/{key}`；新增 `Open` 从 MinIO 或本地读取
2. **后端** `GET /uploads/*filepath` 网关代理（无需 MinIO 公共读策略）
3. **前端** 移除 FormData 请求的手动 `Content-Type`；新增 `mediaUrl()` 重写历史 MinIO URL

## 交付

- `backend/internal/service/file.go` — 统一 URL + `Open`
- `backend/internal/handler/file.go` — `RegisterPublic` / `serve`
- `backend/internal/app/bootstrap.go` — 注册 `/uploads` 代理
- `frontend/src/stores/chat.ts`、`auth.ts` — FormData 不覆盖 Content-Type
- `frontend/src/utils/media.ts` — `mediaUrl` 工具
- `frontend/src/views/ChatView.vue`、`GroupDetailView.vue`、`EditProfileView.vue` — 使用 `mediaUrl`
- `scripts/smoke-api.ps1` — 文件上传/下载冒烟

## 测试

```powershell
cd backend; go build ./...
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

手动验证：
- 上传返回 `/uploads/...`，`GET /uploads/...` → 200
- 旧 MinIO 对象 `334729378686570496_test.txt` 经网关可读

结果：通过（已重启 gateway-http）

## 验证

- [x] 上传 API 返回 `/uploads/{key}`
- [x] 网关代理 MinIO 文件可访问
- [x] 前端 build 通过
- [x] 冒烟脚本含 file upload 段

## 下阶段

Phase 91：按需（喊停即停）；可选为 `/uploads` 加鉴权或签名 URL
