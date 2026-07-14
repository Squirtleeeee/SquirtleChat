# Phase 96 完成

## 需求分析

- 智能体昵称改为「杰尼龟龟」
- 头像使用宝可梦杰尼龟形象

## 设计计划

- 后端 `AgentService.Init` 同步 bot 昵称与头像（PokeAPI 官方立绘 URL）
- 前端静态资源 `public/agent/squirtle.png`（同源加载更快）
- 列表与展示层识别 `squirtle_ai` 时使用本地头像

## 交付

- `backend/internal/service/agent.go` — 昵称、头像同步、提示文案
- `frontend/public/agent/squirtle.png` — 杰尼龟立绘
- `frontend/src/constants/agent.ts` — 常量与头像辅助
- `frontend/src/views/ChatView.vue` — 好友列表头像
- `frontend/src/stores/chat.ts` — 显示名

## 测试

```powershell
cd backend; go build ./...
cd frontend; npm run build
```

## 验证

- [ ] 重启后端后好友列表显示「杰尼龟龟」与杰尼龟头像
- [ ] 聊天标题与助手自称一致

## 下阶段

流式回复、欢迎语等。
