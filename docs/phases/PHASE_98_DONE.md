# Phase 98 完成 — 群聊邀请体系

## 需求分析

- 创建群聊不再输入用户 ID
- 支持从好友列表多选发送入群邀请
- 支持面对面建群（4 位建群码，10 分钟有效）
- 每个群拥有 4 位群号，可搜索并申请加入
- 群主可从好友列表邀请成员
- 所有入群须经邀请 → 接受流程

## 设计计划

- DB：`groups.group_no`、`group_invitations`、`group_face_sessions`
- API：创建群仅含群主；邀请/接受/拒绝；群号搜索；面对面 start/join
- 前端：AddContactModal 重做；侧栏群邀请通知；群详情展示群号与邀请好友

## 交付

### 数据库

- `deploy/init/mysql/005_group_social.sql`

### 后端 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/groups` | `{ name, invite_friend_ids? }` 建群并邀请好友 |
| POST | `/groups/face-to-face/start` | 面对面建群，返回 `face_code` |
| POST | `/groups/face-to-face/join` | 输入建群码，生成入群邀请 |
| GET | `/groups/search-no?q=` | 按群号搜索 |
| GET | `/groups/by-no/:no` | 群公开信息 |
| POST | `/groups/join-by-no` | 通过群号申请加入（生成邀请） |
| GET | `/groups/invitations` | 待处理群邀请 |
| POST | `/groups/invitations/:id/accept` | 接受入群 |
| POST | `/groups/invitations/:id/reject` | 拒绝入群 |
| POST | `/groups/:id/invites` | 群主/管理员邀请好友 |

### 前端

- `AddContactModal.vue` — 选好友建群 / 面对面建群 / 搜群号
- `ChatView.vue` — 侧栏群聊邀请通知
- `GroupDetailView.vue` — 群号展示、邀请好友
- `stores/chat.ts` — 群邀请与建群 API

## 测试

```powershell
cd backend; go build ./...
cd frontend; npm run build
.\scripts\smoke-api.ps1
```

结果：ALL SMOKE TESTS PASSED

## 验证

- [x] 建群仅群主先入群，好友收到邀请
- [x] 接受邀请后加入群成员
- [x] 群号可搜索
- [x] 面对面建群码生成
- [x] 不再使用成员 ID 输入框

## 下阶段

Phase 99：群管理员设置、邀请列表管理、建群码刷新
