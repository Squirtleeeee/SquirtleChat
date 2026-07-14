# SquirtleChat 开发循环

```
开发 -> 测试 -> docs/phases/PHASE_N_DONE.md -> 下一阶段
```

## 启动
```powershell
.\scripts\start-all.ps1          # infra + http + ws
.\scripts\start-dual-ws.ps1      # 双 WS 实例测试
.\scripts\smoke-api.ps1          # API 冒烟（需后端已启动）
cd frontend; npm run dev
```

## 分布式 WS
- 环境变量 `GATEWAY_INSTANCE_ID`（默认 gw-1）
- 环境变量 `WS_PORT`（默认 8081）
- 多实例各用不同 ID + 端口

## 测试清单
- [x] 注册/登录
- [x] WS 连接
- [x] 好友申请接受/拒绝/删除
- [x] 单聊实时 + 历史
- [x] 群聊
- [x] 文件上传 + 聊天展示
- [x] 多端 sync
- [x] 已读/未读角标
- [x] 离线信箱 + sync 补发
