# Phase 57 完成

## 需求分析
- 单行 input 无法写多段消息
- 需要 Enter 发送、Shift+Enter 换行

## 设计计划
- composer 改为自适应高度 textarea（最高 120px）
- Enter（无 Shift）发送；Shift+Enter 换行
- 气泡渲染 `\n` → `<br>`

## 交付
- `frontend/src/views/ChatView.vue`：多行输入与换行显示

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- Shift+Enter 可换行，Enter 发送
- 多行消息在气泡中正确换行
- 草稿仍按会话保存多行内容

## 下阶段
Phase 58：继续（喊停即停）
