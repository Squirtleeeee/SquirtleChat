# Phase 61 完成

## 需求分析
- 图片灯箱、添加好友弹窗、页面路由切换仍是硬切，与 Phase 60 主界面过渡不协调
- 会话搜索结果可能露出引用协议原文

## 设计计划
- 灯箱 / 添加联系人：淡入 + 轻微缩放
- App 级路由 `page` 过渡
- 搜索摘要用 `messageBody` 剥离引用头
- 侧栏删除按钮 opacity 过渡

## 交付
- `frontend/src/components/ImageLightbox.vue`
- `frontend/src/components/AddContactModal.vue`
- `frontend/src/App.vue` — RouterView page transition
- `frontend/src/views/ChatView.vue` — 搜索摘要、模态 Transition、del-btn

## 测试
```powershell
cd frontend; npm run build
```
通过

## 验证
- 打开/关闭图片预览有淡入
- 添加好友弹窗卡片上浮淡入
- 登录 ↔ 聊天 / 资料页切换不硬切
- 搜索命中含引用消息时摘要无协议字符

## 下阶段
Phase 62：列表交互反馈与通知条过渡继续打磨
