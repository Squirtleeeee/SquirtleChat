# Phase 20 完成

## 需求分析
- 好友/群搜索不应占据左侧栏，应通过「+ 添加好友/群聊」弹窗触发
- 主界面以聊天为主；无好友时显示空状态并链接到添加流程
- 核心页不展示用户 ID，仅头像+用户名；详情与编辑资料在点头像后的次级页

## 设计计划
- `AddContactModal`：搜索好友 / 搜索群聊 / 创建群聊 三 Tab
- 精简 `ChatView` 侧栏：好友列表 + 左下角添加按钮
- 路由：`/profile`、`/profile/edit`、`/profile/:id`
- `UserAvatar` 统一头像展示

## 交付
- `frontend/src/components/AddContactModal.vue`
- `frontend/src/components/UserAvatar.vue`
- `frontend/src/views/ChatView.vue` 重构
- `frontend/src/router/index.ts` 资料路由

## 测试
```powershell
npm run build
# 手动：无好友时空状态 → 点击「添加好友」→ 弹窗
# 点击侧栏头像 → 资料页 → 编辑资料
```

## 验证
- 侧栏无搜索框、无用户 ID 文案
- 好友列表显示昵称/用户名与头像

## 下阶段
Phase 21：统一搜索与模糊匹配
