# Phase 94 完成

## 需求分析

- 左侧好友/群列表与右侧聊天记录应**各自独立滚动**，互不影响。
- 点开会话后应默认停在**最新消息（底部）**，而非最早记录处。
- 本阶段不处理杰尼助手 LLM 回复问题。

## 设计计划

- **布局**：`app-shell` / `sidebar` / `chat-main` 固定视口高度并 `overflow: hidden`；列表区域 `flex:1; min-height:0; overflow-y:auto`。
- **防串联滚动**：`overscroll-behavior: contain`；聊天页挂载时锁定 `html` 页面级滚动。
- **滚底**：`scrollToLatestOnOpen()` 在历史加载后多次尝试滚到底部，避免 DOM 未渲染完停在顶部。

## 交付

| 文件 | 变更 |
|------|------|
| `frontend/src/views/ChatView.vue` | 侧栏 `sidebar-list-pane`、独立滚动容器、`scrollToLatestOnOpen` |
| `frontend/src/styles/theme.css` | `.chat-route-lock` 禁止页面级滚动 |

## 测试

```powershell
cd frontend; npm run build
```

## 验证

- [ ] 左侧列表上下滚动时，右侧聊天区位置不变
- [ ] 右侧聊天记录滚动时，左侧列表位置不变
- [ ] 点击好友/群聊打开后，直接看到最新消息（底部）
- [ ] 发送消息后仍跟随滚到底部

## 下阶段

Phase 95：继续排查杰尼助手 LLM；或流式回复。
