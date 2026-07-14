# Phase 14 完成

## 需求分析
- PRD：单聊（文本/图片）、文件上传并在聊天展示
- 后端 `POST /files/upload` 已有，聊天未接入

## 交付
- 前端 📎 按钮上传文件
- 图片消息 `msg_type=2`，文件 `msg_type=3`
- 消息 content 为 JSON：`{url, filename, content_type}`
- 气泡内展示图片预览或文件下载链接

## 测试
- `npm run build` 通过
- `go build ./...` 通过

## 验证
- 上传图片后在聊天气泡显示
- 上传文件显示可点击文件名

## 下阶段
Phase 15: 离线信箱
