# Phase 7 完成

## 交付
- Redis 在线路由 `online:{userID}:{deviceID}` -> instanceID
- 每 gateway 独立 Kafka consumer group
- Redis PubSub 跨实例推送 `ws:push:{instanceID}`
- `pkg/routing` + `internal/push/dispatcher`
- `scripts/start-dual-ws.ps1` 双实例测试

## 验证
1. 启动 infra + 两个 gateway-ws (8081/8082)
2. 用户 A 连 8081，用户 B 连 8082
3. 单聊消息双方实时收到

## 下阶段
Phase 8: MinIO + 健康检查 + 一键启动
