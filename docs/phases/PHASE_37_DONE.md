# Phase 37 完成

## 需求分析
- 群详情成员列表无排序、无用户名、难辨认自己
- 群聊无法 @ 成员，消息中 @ 无高亮

## 设计计划
- 打开群聊时缓存 `groupMembers`
- 群详情：群主优先、自己次之、按昵称排序；显示 `@username` /「我」标签
- 输入 `@` 弹出成员选择；插入 `@昵称 `；气泡内 `@xxx` 高亮

## 交付
- `frontend/src/stores/chat.ts`：`groupMembers`、`mentionName`、打开群时 `fetchGroup`
- `frontend/src/views/GroupDetailView.vue`：成员排序与信息展示
- `frontend/src/views/ChatView.vue`：@ 选择面板、消息高亮

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 群详情成员：群主置顶，显示用户名，自己有「我」
- 群聊输入 `@` 出现成员列表，点选插入
- 发送后气泡中 `@昵称` 高亮显示

## 下阶段
Phase 38：会话免打扰 / 静音（本地偏好 + 通知抑制）
