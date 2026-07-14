# Phase 29 完成

## 需求分析
- 文本消息需要一键复制到剪贴板
- 常用好友/群聊需要置顶，刷新后仍保持顺序

## 设计计划
- 文本气泡悬停显示「复制」按钮，调用 `navigator.clipboard`
- 置顶状态存 `localStorage`（`squirtlechat_pinned`），好友与群聊分开
- 侧栏列表按置顶优先、再按最近消息时间排序

## 交付
- `frontend/src/stores/chat.ts`：`pinnedFriendIds` / `pinnedGroupIds`、`togglePin*`、`isPinned*`
- `frontend/src/views/ChatView.vue`：📌 置顶按钮、复制按钮、置顶高亮样式

## 测试
```powershell
cd backend; go build ./...
cd frontend; npm run build
```

## 验证
- 悬停文本消息点「复制」，顶部提示「已复制」
- 点击 📌 置顶好友/群聊，刷新页面后置顶仍生效
- 置顶会话排在列表最前

## 下阶段
Phase 30：消息撤回（2 分钟内）
