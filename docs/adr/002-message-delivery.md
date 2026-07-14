# ADR-002: 消息投递模型

## 状态
已接受

## 决策
- MySQL 权威存储
- Kafka topic `im.message.sent` 扇出
- Redis 在线路由 `online:{user_id}` -> gateway实例
- client_msg_id 幂等
- conversation_seq 会话内排序

## 原因
标准 IM 路径，支持离线、多端、扩展消费者。
