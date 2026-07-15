# Phase 118 完成 — 语音消息

## 需求分析

- 目标：录制短语音（≤60s）→ 上传 → `msg_type=5` 发送 → 气泡内播放。

## 设计计划

- `MsgTypeAudio = 5`
- `chat.sendVoice(blob, duration)`
- Composer 麦克风按钮 + 录音条；气泡 `<audio controls>`

## 交付

- model.go MsgTypeAudio
- format preview `[语音]`
- chat.sendVoice + ChatView 录音/播放 UI

## 测试

- `go build ./...` + `npm run build` + 后端重启 ✅

## 下阶段

Phase 119：独立聊天窗补齐图片/文件/语音能力（对齐主窗）
