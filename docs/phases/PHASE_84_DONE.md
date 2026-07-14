# Phase 84 完成

## 需求分析

对照微信会话列表与群顶栏：
- 草稿预览用醒目「草稿」标签 + 正文，而非整行同色 `[草稿] …`
- 群公告条用喇叭图标强化识别

## 设计计划

- `draftPreview` 只返回正文；模板加 `.draft-tag`
- `.group-notice-label` 加喇叭 SVG

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 有草稿的会话预览前有橙色「草稿」
- 群公告左侧喇叭 +「公告」

## 下阶段

Phase 85：移动端横向滑动回复（Telegram 式）
