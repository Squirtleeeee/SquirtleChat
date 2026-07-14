# Phase 58 完成

## 需求分析
- 打开有未读的会话时，难以一眼看到「从哪条开始是新消息」

## 设计计划
- 打开会话前记录 `last_read_seq`（仅当 unread_count > 0）
- 渲染时在第一条 `seq > last_read_seq` 且非自己的消息前插入「以下为新消息」

## 交付
- `frontend/src/views/ChatView.vue`：未读分隔线

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 有未读时打开会话，历史与新消息之间出现分隔
- 无未读时不出现
- 自己发送的消息不触发分隔插入点误判

## 下阶段
Phase 59：继续（喊停即停）
