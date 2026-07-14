# Phase 8 完成

## 交付
- FileService 优先 MinIO，失败回退本地 `data/uploads`
- `/health` 含 redis 状态
- `scripts/start-all.ps1` 一键启动

## 验证
- MinIO 启动后上传文件 URL 指向 MinIO
- health 返回 redis: ok

## 项目状态
SquirtleChat MVP 全阶段完成。见 docs/DEV_WORKFLOW.md
