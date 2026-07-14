# Phase 54 完成

## 需求分析
- 移动端无右键，侧栏菜单难触发
- 需长按呼出与桌面右键相同菜单

## 设计计划
- touchstart 480ms 后弹出 ctxMenu
- touchmove 取消；长按后抑制紧随的 click 打开会话

## 交付
- `frontend/src/views/ChatView.vue`：好友/群聊长按菜单

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 手机/模拟器长按好友或群聊弹出菜单
- 滑动取消长按
- 长按后松手不误打开会话

## 下阶段
Phase 55：侧栏一键全部标为已读
