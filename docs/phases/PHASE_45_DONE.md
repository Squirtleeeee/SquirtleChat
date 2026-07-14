# Phase 45 完成

## 需求分析
- 置顶/免打扰入口分散（小图标、会话头按钮），侧栏缺少统一操作入口
- 需要右键菜单：打开会话、置顶、免打扰、查看资料/群信息

## 设计计划
- 好友/群聊列表 `@contextmenu` 弹出固定定位菜单
- 点击外部或 Esc 关闭
- 复用现有 `togglePin*` / `toggleMute` / 路由跳转

## 交付
- `frontend/src/views/ChatView.vue`：右键菜单 UI 与操作

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 右键好友：置顶 / 免打扰 / 查看资料可用
- 右键群聊：置顶 / 免打扰 / 群聊信息可用
- Esc 或点击空白关闭菜单

## 下阶段
Phase 46：冒烟测试扩展（消息搜索有内容命中）+ README Phase 索引更新
