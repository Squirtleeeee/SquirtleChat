# Phase 103–105 完成 — 桌面软件 + 排版模式

## 需求分析

- 项目需以桌面软件形式运行（类似微信 PC）
- 设置中可切换排版：
  - **经典单窗口**：侧栏 + 右侧聊天区（现状）
  - **会话独立窗口**：点击会话弹出独立小窗（参考微信「独立窗口显示」）
- 任意模式下右键可「独立窗口打开」
- 桌面端支持托盘、聊天窗置顶

## 设计计划

- Electron 壳：`desktop/main.cjs` + `preload.cjs`
- 设置：`stores/settings.ts` + `SettingsView`
- 独立窗：`/popup-chat` + IPC `open-chat-window`
- 启动：`scripts/start-desktop.ps1`

## 交付

| 能力 | 说明 |
|------|------|
| 桌面壳 | Electron 主窗口 + 系统托盘 |
| 设置 | `/settings` 切换 embedded / detached |
| 独立窗 | 微信式弹出聊天窗，可置顶 |
| 网页兼容 | 无 Electron 时用 `window.open` 兜底 |

## 测试

```powershell
cd frontend; npm run build
cd desktop; npm install
.\scripts\start-desktop.ps1
```

## 下阶段

Phase 106：独立窗完整消息能力（图片/文件）、窗口位置记忆、开机启动
