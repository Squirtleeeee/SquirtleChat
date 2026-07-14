# Phase 55 完成

## 需求分析
- 未读会话多时需逐个点开才能清角标
- 需要一键将全部未读会话标为已读

## 设计计划
- `markAllRead`：对 `unread_count > 0` 的会话批量 `POST /sync/read`（`read_seq = last_seq`）
- 侧栏 Tab 旁「全读」按钮（有未读时显示）

## 交付
- `frontend/src/stores/chat.ts`：`markAllRead`
- `frontend/src/views/ChatView.vue`：全读按钮

## 测试
```powershell
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 有未读时侧栏出现「全读」
- 点击后角标清零，提示已处理会话数
- 无未读时提示「没有未读消息」

## 下阶段
Phase 56：群聊气泡显示发送者昵称
