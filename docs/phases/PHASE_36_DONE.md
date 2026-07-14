# Phase 36 完成

## 需求分析
- 切换会话后输入框清空，未发送内容丢失
- 刷新页面后草稿也不保留

## 设计计划
- `localStorage` 键 `squirtlechat_drafts`：`{ [conversation_id]: text }`
- 输入时写入；切换会话先保存旧草稿再恢复新会话草稿；发送成功清除
- 侧栏预览优先显示 `[草稿] …`

## 交付
- `frontend/src/views/ChatView.vue`：草稿读写、切换恢复、侧栏草稿预览

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 会话 A 输入未发送 → 切到 B → 再回 A，输入框内容仍在
- 刷新页面后草稿仍在
- 发送成功后草稿清除，侧栏不再显示 `[草稿]`

## 下阶段
Phase 37：群成员列表展示优化 / @提及（基础）
