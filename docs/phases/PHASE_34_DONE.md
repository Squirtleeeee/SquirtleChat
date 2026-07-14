# Phase 34 完成

## 需求分析
- 会话内无法按关键词查找历史文本
- 搜索命中后无法跳转到消息所在位置（仅加载最近 50 条）

## 设计计划
- `GET /conversations/:id/messages/search?q=`：成员校验 + 文本 `LIKE` 搜索
- `GET /conversations/:id/messages?around_seq=`：按 seq 拉取附近消息用于定位
- 列表接口补成员校验；好友单聊懒创建会话
- 前端：会话头「搜索」面板、结果列表、跳转高亮

## 交付
- `backend/internal/store/message.go`：`SearchInConversation`、`ListAround`
- `backend/internal/service/message.go`：`SearchMessages`、成员校验、`around_seq`
- `backend/internal/handler/message.go`：search 路由
- `frontend/src/stores/chat.ts`：`searchMessages`、`jumpToMessage`、高亮
- `frontend/src/views/ChatView.vue`：搜索面板与定位滚动
- `docs/api/HTTP.md`、`scripts/smoke-api.ps1`：文档与冒烟覆盖

## 测试
```powershell
cd backend; go build ./...
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

## 验证
- 打开会话点「搜索」，输入关键词出现匹配文本消息
- 点击结果滚动到对应气泡并短暂高亮
- 命中不在当前已加载窗口时，通过 `around_seq` 补齐后仍可定位
- 非会话成员搜索返回 403

## 下阶段
Phase 35：输入中状态（typing）/ 对方正在输入提示
