# Phase 119 完成 — 独立窗媒体对齐

## 需求分析

- 现状：`ChatPopupView` 仅支持纯文本发送；图片仅靠 JSON content 粗略识别；无文件链接、语音气泡、上传进度。
- 缺口：与主窗 `ChatView` 的图片/文件/语音收发不一致，独立窗无法作为完整会话表面。
- 验收：独立窗可发图/文件/语音；气泡正确展示；粘贴图片可发；图片可灯箱预览。

## 设计计划

- 复用 `chat.uploadAndSend` / `chat.sendVoice`
- Composer：附件 + 麦克风 + 录音条
- 消息渲染：`msg_type` 2/3/5 + 上传 overlay + 文件下载链
- 粘贴图片、灯箱预览；不引入回复/表情等完整工具栏（下阶段可选）

## 交付

- `frontend/src/views/ChatPopupView.vue`：媒体发送与展示对齐主窗

## 测试

- `npm run build` ✅

## 验证

- [ ] 独立窗发送图片/文件
- [ ] 录音并播放
- [ ] 粘贴截图发送
- [ ] 点击图片灯箱

## 下阶段

Phase 120：会话内消息置顶（pin in-thread）
