# Phase 22 完成

## 需求分析
- 聊天存在延迟感，疑似 WS 重连/token/ack 等问题

## 设计计划
- WS 重连使用最新 token（`squirtle:token-refreshed` 事件）
- 处理 `ack` 帧更新本地消息 `seq`
- 发送失败时提示并重连；`pullSync` 间隔 10s → 3s
- `bindWS` 防止重复注册 handler

## 交付
- `frontend/src/api/ws.ts`：`refreshToken`、`onTokenRefresh`、发送返回值
- `frontend/src/api/http.ts`：refresh 后派发 token 事件
- `frontend/src/stores/chat.ts`：ack 合并、`mergeMessage` 更新、发送后 pullSync

## 测试
```powershell
go build ./...
npm run build
# 双账号互发消息，观察是否即时出现
```

## 验证
- Token 刷新后 WS 重连不再 401 静默失败
- 自己发送的消息收到 ack 后带有 seq

## 项目状态
资料/隐私、简化 UI、统一搜索、实时优化已交付。开发循环 skill：`.cursor/skills/squirtle-dev-cycle/SKILL.md`
