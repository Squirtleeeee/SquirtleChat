# Phase 41 完成

## 需求分析
- 消息中的 `http(s)://` URL 为纯文本，无法点击打开
- 需转义防 XSS，并剥离尾部标点

## 设计计划
- `renderMessageHtml`：先 escape，再识别 URL 为 `<a target="_blank" rel="noopener">`，再高亮 `@mention`
- 样式：主色下划线、`word-break`

## 交付
- `frontend/src/views/ChatView.vue`：链接识别与样式

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 发送含 `https://example.com` 的消息，气泡内可点击新开页
- URL 后紧跟句号时句号不进入链接
- `<script>` 等文本被转义，不执行

## 下阶段
Phase 42：键盘快捷键（Esc 关闭面板、Ctrl+Enter 发送）
