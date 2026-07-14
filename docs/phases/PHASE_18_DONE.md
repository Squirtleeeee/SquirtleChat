# Phase 18 完成

## 需求分析
- access_token 过期后前端需手动重新登录
- 缺少端到端 API 回归脚本

## 交付
- `frontend/src/api/http.ts`：401 时自动 refresh 并重试
- `scripts/smoke-api.ps1`：注册/搜索/好友/资料/群/会话 冒烟
- `internal/service/auth_test.go` 参数校验单测

## 测试
```powershell
.\scripts\start-backend.ps1
.\scripts\smoke-api.ps1
go test ./...
npm run build
```

## 验证
- smoke 脚本输出 `ALL SMOKE TESTS PASSED`
- token 过期后 API 自动续期（需 refresh_token 有效）

## 项目状态
MVP + 用户资料/搜索 + proto 契约 + 冒烟测试已就绪。
