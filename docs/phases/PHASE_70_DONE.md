# Phase 70 完成

## 需求分析
- 系统默认滚动条偏粗，与整体视觉不协调
- 引用气泡只能看、不能跳到原消息，交互不完整

## 设计计划
- 全局细滚动条 + 选区高亮品牌色
- 点击引用块平滑滚动并高亮原消息（已加载范围内）

## 交付
- `frontend/src/styles/theme.css` — scrollbar / selection
- `frontend/src/views/ChatView.vue` — `jumpToQuoted`

## 测试
```powershell
cd frontend; npm run build
```
通过

## 验证
- 侧栏/消息列表滚动条细且悬停加深
- 点击引用块定位并高亮原消息；超出范围有提示

## 下阶段
Phase 71：继续（喊停即停）
