# Phase 26 完成

## 需求分析
- 长聊天记录缺少日期分割（今天/昨天），不便浏览
- 好友/群较多时侧栏无法快速筛选

## 设计计划
- `formatDateDivider` + `dayKey` 在消息列表插入日期条
- 侧栏顶部搜索框，过滤 `sortedFriends` / `sortedGroups`

## 交付
- `frontend/src/utils/format.ts`：`formatDateDivider`、`dayKey`
- `frontend/src/views/ChatView.vue`：`displayItems`、侧栏 `filterQuery`

## 测试
```powershell
npm run build
```

## 验证
- 跨天消息之间显示「今天」「昨天」或日期标签
- 侧栏输入关键词可过滤好友昵称/用户名或群名

## 下阶段
Phase 27：好友备注 + 图片消息点击预览
