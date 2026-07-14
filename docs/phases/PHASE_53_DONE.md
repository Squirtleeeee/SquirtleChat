# Phase 53 完成

## 需求分析
- 本地消息缓存膨胀或脏数据时，需要一键清空当前会话本地列表
- 不能误删服务端历史

## 设计计划
- `clearLocalMessages`：清空 `messages[convId]`，重置 `historyHasMore`
- 会话头「清空」二次确认
- 空列表时提供「重新加载历史」

## 交付
- `frontend/src/stores/chat.ts`：`clearLocalMessages`、`reloadActiveHistory`
- `frontend/src/views/ChatView.vue`：清空按钮与空态重载

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 点「清空」确认后消息区变空
- 「重新加载历史」可拉回服务端消息
- 服务端数据不受影响

## 下阶段
Phase 54：好友/群列表长按（移动端）呼出与右键相同菜单
