# Phase 83 完成

## 需求分析

对照 WhatsApp 送达状态序列：发送中时钟 → 单勾已发送 → 双勾已读。
当前 sending 仅降低气泡透明度，无状态图标。

验收：sending/uploading 显示时钟 SVG；build 通过。

## 设计计划

- 自己消息 `status === sending|uploading` 优先显示 `.read-tag.pending` 时钟
- 再回落已读双勾 / 已发送单勾

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 刚发出的消息右下角为时钟，ack 后变单勾，对方已读变双勾

## 下阶段

Phase 84：群公告图标 + 草稿标识对齐主流 IM
