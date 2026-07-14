# Phase 44 完成

## 需求分析
- 气泡时间仅到分钟，排查时不够精确
- 复制需悬停点按钮，双击更快捷

## 设计计划
- 点击时间切换 HH:mm ↔ HH:mm:ss（`formatMessageTime(iso, showSeconds)`）
- 双击文本气泡复制内容，短暂提示「已复制」
- 切换会话清除精确时间状态

## 交付
- `frontend/src/utils/format.ts`：`showSeconds` 参数
- `frontend/src/views/ChatView.vue`：时间切换、双击复制

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 点击气泡时间显示秒，再点恢复
- 双击文本消息复制成功并提示
- 图片消息双击不触发复制

## 下阶段
Phase 45：侧栏会话右键菜单（置顶 / 免打扰 / 打开资料）
