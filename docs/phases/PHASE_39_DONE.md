# Phase 39 完成

## 需求分析
- 图片消息立即加载全部 src，长会话滚动卡顿
- 上传文件无进度，大图发送时用户不知是否卡住

## 设计计划
- `<img loading="lazy" decoding="async">`
- 上传前本地 blob 预览 + `onUploadProgress` 更新百分比
- 气泡遮罩进度条 + composer 上方上传横幅；上传中禁用发送/附件

## 交付
- `frontend/src/stores/chat.ts`：`uploadAndSend` 进度与本地预览
- `frontend/src/views/ChatView.vue`：懒加载、进度 UI

## 测试
```powershell
cd frontend; npm run build
```

## 验证
- 历史图片滚动时按需加载（Network 可见 lazy）
- 发送图片时先出现本地预览与进度百分比
- 上传完成后进度消失，消息正常发出

## 下阶段
Phase 40：表情面板（常用 emoji）/ 快捷插入
