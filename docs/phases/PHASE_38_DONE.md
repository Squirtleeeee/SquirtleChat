# Phase 38 完成

## 需求分析
- 群聊/单聊无法静音，桌面通知与 Tab 未读角标无法抑制
- 需要本地偏好即可，暂不要求服务端同步

## 设计计划
- `localStorage` 键 `squirtlechat_muted`：会话 ID 列表
- 静音会话：不弹桌面通知；Tab 未读汇总排除；侧栏显示 🔕 替代数字角标
- 会话头「免打扰 / 已静音」切换

## 交付
- `frontend/src/stores/chat.ts`：`mutedConvIds`、`toggleMute`、通知/未读过滤
- `frontend/src/views/ChatView.vue`：免打扰按钮、侧栏静音标记

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 打开会话点「免打扰」，侧栏该会话显示 🔕
- 静音后新消息不弹桌面通知，Tab 角标不计入
- 再点「已静音」恢复通知与角标

## 下阶段
Phase 39：图片消息缩略图懒加载 / 发送进度提示
