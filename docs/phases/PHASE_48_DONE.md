# Phase 48 完成

## 需求分析
- 上翻历史时新消息仍强制滚到底，打断阅读
- 离开底部后无入口快速回到最新

## 设计计划
- 滚动检测 `nearBottom`（阈值 80px）
- 仅在底部时自动跟随新消息；否则累计 `pendingNewCount`
- 浮动按钮：「回到底部」/「N 条新消息」

## 交付
- `frontend/src/views/ChatView.vue`：滚动跟随策略 + 跳转按钮

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 上翻历史后新消息不自动跳底，出现浮动按钮
- 按钮显示新消息条数；点击滚到底并清零
- 在底部时行为与原先一致（自动跟随）

## 下阶段
Phase 49：登录页记住用户名 / 退出确认
