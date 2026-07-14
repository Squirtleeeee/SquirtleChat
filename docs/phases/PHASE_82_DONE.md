# Phase 82 完成

## 需求分析

对照 WhatsApp / Telegram 失败与断线反馈：
- 发送失败用警示图标 + 短文案「重试」，而非长句纯文字链
- 重连条应有状态图标，一眼可辨「网络问题」

验收：失败重试按钮含 SVG；重连条含刷新图标；build 通过。

## 设计计划

- `.retry-btn` inline-flex：圆叹号图标 +「重试」
- `.reconnect-banner-main` + 双向箭头刷新 SVG

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 失败消息旁显示红色警示图标与「重试」
- 断线/同步横幅左侧有重连图标

## 下阶段

Phase 83：发送中时钟图标（WhatsApp 式送达前状态）
