# SquirtleChat 技术职责边界

| 层 | 技术 | 职责 |
|----|------|------|
| 客户端实时 | WebSocket | 上行消息、下行推送、心跳 |
| 客户端管理 | HTTP REST | 登录、好友、群、历史拉取、sync |
| 服务间 | gRPC | 同步 RPC |
| 权威存储 | MySQL | 用户/关系/消息/群 |
| 事件总线 | Kafka | 消息事件、推送、同步事件 |
| 状态缓存 | Redis | Session、在线路由、未读、幂等 |
| 文件 | MinIO | 对象存储 |

## 禁止
- 客户端直连 gRPC
- Redis 作主消息队列
- Kafka 作历史查询源
- 文件存 MySQL BLOB
