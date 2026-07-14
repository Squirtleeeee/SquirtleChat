# SquirtleChat 架构

## 目录
```
SquirtleChat/
├── docs/
├── deploy/
├── scripts/
├── backend/          # Go 微服务
│   ├── api/proto/
│   ├── pkg/
│   └── services/
└── frontend/         # Vue3 SPA
```

## 服务
| 服务 | 端口(默认) | 说明 |
|------|-----------|------|
| gateway-http | 8080 | REST API |
| gateway-ws | 8081 | WebSocket |
| auth | 9001 | 认证 gRPC |
| user | 9002 | 用户 gRPC |
| friend | 9003 | 好友/群 gRPC |
| message | 9004 | 消息 gRPC |
| push | - | Kafka 消费者 |
| file | 9005 | 文件 gRPC |

## 消息流（单聊）
```
Client --WS--> gateway-ws --gRPC--> message
  -> MySQL 持久化
  -> Kafka im.message.sent
  -> push 消费 -> Redis 查路由 -> gateway-ws 下行
  -> 离线写 offline_inbox
```

## ID 策略
- Snowflake: msg_id, 全局 ID
- conversation_id: 单聊=min(uid1,uid2)_max(uid1,uid2); 群=group_id
- conversation_seq: 会话内单调递增

## 多端同步
- 每用户每设备: sync_cursor(sync_seq)
- 重连: GET /api/v1/sync?since_seq=N
