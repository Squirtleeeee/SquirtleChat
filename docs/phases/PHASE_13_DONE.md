# Phase 13 完成

## 需求分析
- PRD：群聊（创建/邀请/退群/群消息）
- 此前仅 `POST /groups` 后端，无列表与前端

## 交付
- `GET /groups` 群列表
- `GET /groups/:id` 群详情+成员
- `POST /groups/:id/members` 邀请成员（群主）
- `DELETE /groups/:id/members/:uid` 退群
- 前端：群聊 Tab、建群表单、群会话、群消息 WS 发送

## 测试
- `go build ./...` 通过
- `npm run build` 通过

## 验证
- 创建群后出现在群列表
- 群成员可收发群消息（多账号）

## 下阶段
Phase 14: 文件/图片聊天
