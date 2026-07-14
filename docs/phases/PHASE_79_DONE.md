# Phase 79 完成

## 需求分析

对照 WhatsApp / Telegram：
- 会话列表未读行有轻背景强调（不只角标）
- 已读用双勾图标，已发送用单勾，而非纯文字「已读」

## 设计计划

- `.friend-item.has-unread` 浅绿底
- `read-tag` SVG 单/双勾

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 未读会话行有淡青绿底 + 加粗标题
- 自己消息：送达单勾，对方已读双勾

## 下阶段

Phase 80：继续（喊停即停）
