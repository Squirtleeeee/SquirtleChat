# Phase 21 完成

## 需求分析
- 用户名与 ID 应同一搜索框
- 无精确用户名匹配时，按相似度返回候选用户
- 群聊也需可搜索

## 设计计划
- `UserStore.Search`：精确 ID/用户名优先；否则 LIKE + Levenshtein 排序
- 搜索 API 返回 `PublicProfile`（已隐私过滤）
- `GET /groups/search`：按群名搜索当前用户已加入的群
- `GET /friends` 返回好友 `PublicProfile` 列表

## 交付
- `backend/internal/store/user.go`：`searchScore`、`levenshtein`
- `backend/internal/handler/friend.go`：`searchGroups`、好友列表 enriched
- `frontend/src/stores/chat.ts`：`searchGroups`、`loadFriends` 新结构

## 测试
```powershell
.\scripts\smoke-api.ps1
# 搜索 test → 返回多个相似用户
```

## 验证
- 输入完整 ID 可精确找到用户
- 输入部分用户名返回相似结果排序

## 下阶段
Phase 22：实时消息可靠性
