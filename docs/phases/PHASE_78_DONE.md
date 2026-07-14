# Phase 78 完成

## 需求分析

行业惯例（微信 / WhatsApp）：
- 移动端对气泡 **长按** 弹出操作菜单，而非依赖 hover
- 桌面端 **右键** 同菜单
- Hover 操作条仅适合键鼠，触控设备不可靠（ui-ux-pro-max: hover-vs-tap）

## 设计计划

- 气泡 `contextmenu` + 长按约 480ms → `msgMenu`
- 菜单：回复 / 复制 / 撤回 / 多选
- 触控设备隐藏桌面 hover `msg-actions`

## 交付

- `frontend/src/views/ChatView.vue`

## 测试

```powershell
cd frontend; npm run build
```

通过（清理 tsbuildinfo 后）

## 验证

- 桌面：右键气泡出现菜单
- 手机/触控：长按气泡出现菜单；无 hover 操作条
- Esc / 点击空白关闭

## 下阶段

Phase 79：继续对齐（会话列表未读底色 / 已读双勾等）
