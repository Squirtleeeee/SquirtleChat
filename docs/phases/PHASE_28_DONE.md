# Phase 28 完成

## 需求分析
- 用户习惯截图后直接 Ctrl+V 发图，当前只能点附件按钮

## 设计计划
- 输入框 `paste` 事件检测剪贴板图片
- 复用 `chat.uploadAndSend`

## 交付
- `frontend/src/views/ChatView.vue`：`onComposerPaste`

## 测试
```powershell
npm run build
# 截图后于输入框 Ctrl+V
```

## 验证
- 粘贴图片自动上传并作为图片消息发送

## 下阶段
Phase 29：消息复制 + 会话置顶
