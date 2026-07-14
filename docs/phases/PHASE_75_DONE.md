# Phase 75 完成

## 需求分析
- Toast 挂在 ChatView 内，离开聊天页（资料/编辑）后 KeepAlive 停用导致提示不显示

## 设计计划
- Toast 上移到 `App.vue`，全局可用
- ChatView 去掉重复 Toast

## 交付
- `frontend/src/App.vue`
- `frontend/src/views/ChatView.vue`

## 测试
```powershell
cd frontend; npm run build
```
通过

## 验证
- 聊天页复制仍有 Toast
- 编辑资料保存成功后，在资料页也能看到 Toast

## 下阶段
Phase 76：继续（喊停即停）
