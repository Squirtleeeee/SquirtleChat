# Phase 85 完成

## 需求分析

联网对照 Telegram / Stream Chat：
- 气泡**右滑**超过阈值进入回复，左侧出现回复箭头反馈
- 需与长按菜单并存：垂直滑动取消；水平滑动取消长按

验收：触摸右滑可回复；不破坏长按菜单；build 通过。

## 设计计划

- `bubbleSwipeMode` pending/active + `swipeOffset` / `swipeMsgId`
- 阈值 56px，最大位移 72px
- `.swipe-reply-hint` + bubble `translateX`
- `touch-action: pan-y` 优先纵向滚动

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过

## 验证

- 手机/触控：右滑气泡 → 回复栏出现
- 长按仍弹出操作菜单；上滚列表不误触发回复

## 下阶段

Phase 86：`prefers-reduced-motion` 减弱动效（无障碍）
