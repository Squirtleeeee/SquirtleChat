# Phase 31 完成

## 需求分析
- README 信息过少，新开发者难以快速上手
- 冒烟测试未覆盖好友备注 API

## 设计计划
- 扩充 README：功能列表、启动步骤、测试账号、主要 API、迁移说明
- smoke-api 增加 `PUT /friends/:id/remark` 校验（ASCII 备注避免 PowerShell 编码问题）

## 交付
- `README.md`：完整快速启动与 API 索引
- `scripts/smoke-api.ps1`：friend remark 步骤

## 测试
```powershell
.\scripts\smoke-api.ps1
```

## 验证
- 冒烟测试 ALL PASSED（含 friend remark）
- README 可指导从零启动项目

## 下阶段
Phase 32：群聊已读汇总 / 消息长按菜单优化
