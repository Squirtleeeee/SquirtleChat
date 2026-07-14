# Phase 51 完成

## 需求分析
- 群聊缺少公告能力，重要通知只能靠聊天刷屏
- 群主应可设置，成员在会话顶栏可见

## 设计计划
- DB：`groups.notice VARCHAR(500)`（`004_group_notice.sql`）
- `PUT /groups/:id/notice`：仅群主，≤200 字
- 群详情编辑；打开群聊顶栏展示公告条

## 交付
- `deploy/init/mysql/004_group_notice.sql`
- `backend/internal/store|service|handler`：notice 读写
- `frontend`：群详情编辑 + 会话顶栏公告
- smoke / API / README 更新

## 测试
```powershell
cd backend; go build ./...
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 群主可设置/清空公告
- 非群主设置返回业务错误码
- 打开群聊顶栏显示公告摘要

## 下阶段
Phase 52：消息多选转发（基础：复制到剪贴板 / 选中计数）
