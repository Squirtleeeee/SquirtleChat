# Phase 32 完成

## 需求分析
- 侧栏「好友 / 群聊」Tab 无法一眼看出未读总数
- 消息「复制」「撤回」按钮分散，交互不统一

## 设计计划
- Store 增加 `friendsUnread` / `groupsUnread` getter
- Tab 标签显示红色未读角标（99+ 封顶）
- 气泡悬停显示统一 `msg-actions` 操作栏

## 交付
- `frontend/src/stores/chat.ts`：未读汇总 getter
- `frontend/src/views/ChatView.vue`：Tab 角标、消息操作栏样式

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 有未读单聊时「好友」Tab 显示数字角标
- 悬停消息气泡显示「复制」「撤回」操作栏

## 下阶段
Phase 33：连接断开自动重连提示优化 / 离线消息拉取
