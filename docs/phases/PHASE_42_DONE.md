# Phase 42 完成

## 需求分析
- 打开搜索/表情/预览后只能点按钮关闭
- 需要 Ctrl/Cmd+Enter 作为发送快捷键（与 Enter 并存）

## 设计计划
- 全局 `keydown`：Esc 依次关闭表情 → 搜索 → @面板 → 添加弹窗 → 图片预览
- Ctrl/Cmd+Enter 发送；输入框 Enter（无修饰键）发送
- `onUnmounted` 移除监听

## 交付
- `frontend/src/views/ChatView.vue`：快捷键处理

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- Esc 关闭表情/搜索/@/弹窗/预览
- Enter 与 Ctrl+Enter 均可发送
- 离开聊天页后快捷键不再触发

## 下阶段
Phase 43：空状态与加载骨架优化 / 会话列表加载反馈
