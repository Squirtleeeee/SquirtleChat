# Phase 15 完成

## 需求分析
- 架构：离线写 `offline_inbox`
- 表已建，`AddOffline` 从未调用
- 验收：离线后上线 sync 收到消息

## 交付
- 推送时用户无在线 WS 路由 → `offline_inbox` 写入
- `GET /sync` 合并离线消息并清除 inbox
- `Hub.HasUser` 判断本机在线

## 测试
- `go test ./pkg/auth ./pkg/idgen` 通过
- `go build ./...` 通过

## 验证
- 用户完全离线时消息写入 offline_inbox
- 重新登录 pullSync 可收到离线期间消息

## 项目状态
PRD MVP 功能已全部落地（除 gRPC 微服务拆分属 ADR 后续演进）。
见 `docs/DEV_WORKFLOW.md` 测试清单。
