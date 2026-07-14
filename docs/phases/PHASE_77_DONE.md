# Phase 77 完成

## 需求分析

对照主流 IM（微信 / WhatsApp / Telegram）与行业指南（CometChat、Muzli Chat UI）：

| 行业惯例 | 改前 SquirtleChat | 本阶段目标 |
|---------|-------------------|-----------|
| 同一发送者连续消息合并展示（密间距、昵称/时间不重复） | 每条独立气泡、群昵称重复 | 2 分钟内同发送者聚类 |
| 会话列表未读加粗强调 | 仅角标 | 名称/预览加粗 |
| 聊天区轻纹理背景衬托气泡 | 偏平、对比弱 | 点阵底纹 |
| 工具栏用图标而非 emoji 字符 | 😊📎 | SVG 描边图标 |
| 输入框简短引导文案 | 过长 placeholder | 「发消息…」 |

参考：
- [CometChat Chat App Design Best Practices](https://www.cometchat.com/blog/chat-app-design-best-practices)（气泡区分、时间戳、连续消息堆叠、composer、背景）
- [Muzli Chat UI](https://muz.li/inspiration/chat-ui/)（层次、时间折叠）
- 微信气泡宽度约屏宽 70%、图片比例阈值等实践

Out of scope：消息反应、语音消息、深色模式（后续）

## 设计计划

- `displayItems` 增加 `cluster` / `showSender` / `showTime`
- 列表 `has-unread` 样式
- `.message-list` 点阵背景 + 聚类间距
- composer SVG + placeholder 精简

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 同一人连发多条：中间条更紧凑，群昵称只在首条，时间在末条
- 有未读会话名称加粗
- 聊天区可见细点底纹；表情/附件为图标

## 下阶段

Phase 78：继续对齐行业惯例（气泡长按菜单 / 已读勾等）
