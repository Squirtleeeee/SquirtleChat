# SquirtleChat

仿微信即时通讯，Go 微服务后端 + Vue3 前端。

## 功能概览

- 注册 / 登录 / JWT + WebSocket 实时消息
- 好友申请、备注、删除；群聊创建与加入
- 用户资料（头像裁剪、性别、生日、隐私设置）
- 统一搜索（用户名 / ID、模糊匹配）
- 单聊已读回执、桌面通知、消息撤回（2 分钟内）
- 会话内消息搜索与定位、输入中提示、断线重连与离线同步
- 草稿本地保存、免打扰、群聊 @提及、表情面板、链接可点
- 会话置顶、右键菜单、消息复制、图片粘贴发送（进度）、图片预览（懒加载）
- 加载骨架、键盘快捷键（Esc / Ctrl+Enter）
- **杰尼龟龟**：内置 AI 好友，日常陪聊（需配置 `LLM_API_KEY`）

## 项目结构

| 目录 | 说明 |
|------|------|
| `backend/` | Go 网关（HTTP :8080、WS :8081） |
| `frontend/` | Vue3 + Pinia SPA |
| `deploy/` | Docker Compose、MySQL 初始化脚本 |
| `docs/phases/` | 分阶段交付记录（Phase 0–54） |
| `scripts/` | 启动、冒烟测试脚本 |

## 快速启动（本机 MySQL）

```powershell
# 1. 初始化数据库（首次）
.\scripts\setup-local-mysql.ps1 -RootPassword "你的root密码"

# 2. 一键后台启动（后端 + 前端，无弹窗）
.\scripts\start-dev.ps1

# 或启动桌面软件（Electron）
.\scripts\start-desktop.ps1

# 停止
.\scripts\stop-dev.ps1
```

浏览器打开 `http://localhost:5173`，或使用桌面版窗口。日志在 `logs/`。

**排版模式**（设置页）：经典单窗口 / 会话独立窗口（类似微信 PC 独立聊天窗）。

也可分步启动：

```powershell
.\scripts\start-backend.ps1    # HTTP :8080 + WS :8081（后台）
.\scripts\start-frontend.ps1   # Vite :5173（后台）
# 需要看控制台输出时加 -Visible
```

### 测试账号（本地）

| 用户名 | 密码 |
|--------|------|
| test_a | test1234 |
| test_b | test1234 |

### 杰尼龟龟（AI 陪聊）

1. 复制 `deploy/llm.env.example` 为 `deploy/llm.env`
2. 填入 `LLM_API_KEY`（DeepSeek 示例见 `deploy/llm.env.example`）
3. 重启后端：`.\scripts\start-backend.ps1`（**必须重启 gateway-ws** 才生效）

DeepSeek 官方：`LLM_API_BASE=https://api.deepseek.com`，`LLM_MODEL=deepseek-v4-flash`

自检：`GET /api/v1/agent/info` 应显示 `llm: true` 且 `llm_base` 为 `https://api.deepseek.com`（不是 openai.com）。

## Docker 启动

```bash
cd deploy && docker compose up -d
```

## 开发命令

```powershell
# 后端编译
cd backend; go build ./...

# 前端构建
cd frontend; npm run build

# API 冒烟测试（需后端已启动）
.\scripts\smoke-api.ps1
```

## 后端服务

```bash
cd backend
go run ./services/gateway-http   # REST API
go run ./services/gateway-ws     # WebSocket
```

分布式 WS 第二实例：

```bash
GATEWAY_INSTANCE_ID=gw-2 WS_PORT=8082 go run ./services/gateway-ws
```

## 主要 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/auth/register` | 注册 |
| POST | `/auth/login` | 登录 |
| GET | `/users/search` | 搜索用户 |
| GET/PUT | `/users/me` | 个人资料 |
| POST | `/friends/request` | 发起好友申请 |
| PUT | `/friends/:id/remark` | 设置备注 |
| GET | `/conversations` | 会话列表 |
| GET | `/conversations/:id/messages/search` | 会话内消息搜索 |
| POST | `/conversations/:id/messages/:msg_id/recall` | 撤回消息 |
| WS | `/ws` | 实时消息（send / ack / read / recall / typing） |

## 数据库迁移

`deploy/init/mysql/` 下按序号执行：

- `001_init.sql` — 基础表
- `002_profile_privacy.sql` — 资料与隐私
- `003_friend_remark.sql` — 好友备注
- `004_group_notice.sql` — 群公告
- `005_group_social.sql` — 群号、群邀请、面对面建群
- `006_group_no_ten.sql` — 群号改为约 10 位数字

## 阶段索引（近期）

| Phase | 内容 |
|-------|------|
| 33 | WS 重连退避 + 离线同步提示 |
| 34 | 会话内消息搜索与定位 |
| 35 | 输入中（typing）提示 |
| 36 | 消息草稿本地持久化 |
| 37 | 群成员列表 + @提及 |
| 38 | 会话免打扰 |
| 39 | 图片懒加载 + 上传进度 |
| 40 | 表情面板 |
| 41 | URL 可点击 |
| 42 | Esc / Ctrl+Enter 快捷键 |
| 43 | 侧栏/消息加载骨架 |
| 44 | 时间精确到秒、双击复制 |
| 45 | 侧栏右键菜单 |
| 46 | 冒烟扩展 + README 索引 |
| 47 | 移动端侧栏抽屉 |
| 48 | 回到底部 / 新消息浮动按钮 |
| 49 | 记住用户名 + 退出确认 |
| 50 | 全量回归与阶段汇总 |
| 51 | 群公告 |
| 52 | 消息多选复制转发文本 |
| 53 | 清空本地消息缓存 |
| 54 | 移动端长按侧栏菜单 |
| 55 | 一键全部标为已读 |
| 56 | 群聊气泡发送者昵称 |
| 57 | 多行输入 Enter/Shift+Enter |
| 58 | 未读「以下为新消息」分隔线 |
| 59 | 消息引用回复（content 内嵌协议） |
| 60 | UX 流畅度：滚动锚点、面板过渡、去卡顿 |
| 61 | 灯箱/弹窗/路由过渡 + 搜索摘要清理 |
| 62 | 通知条/侧栏/按钮交互反馈 |
| 63 | 发送态/失败态/空态过渡 |
| 64 | 输入中点点动画 + 分隔线淡入 |
| 65 | 资料页/群详情骨架加载态 |
| 66 | 侧栏 Tab / 好友申请列表过渡 |
| 67 | 登录页字段过渡与提交 spinner |
| 68 | 编辑资料隐私面板/保存/裁剪过渡 |
| 69 | KeepAlive 聊天页，返回无闪烁 |
| 70 | 细滚动条 + 点击引用跳转原消息 |
| 71 | 防连点发送 + 图片淡入 |
| 72 | 应用内确认框替代原生 confirm |
| 73 | 成功提示改为底部 Toast |
| 74 | 未读角标轻入场动画 |
| 75 | Toast 提升到 App 全局 |
| 76 | 会话搜索打开后自动聚焦 |
| 77 | 对齐主流 IM：消息合并/未读加粗/底纹/SVG 工具栏 |
| 78 | 气泡长按/右键操作菜单（微信式） |
| 79 | 未读会话底色 + 已读双勾图标 |
| 80 | 侧栏静音/置顶改为 SVG（去 emoji 图标） |
| 81 | 粘性日期分隔 + 文件气泡 SVG |
| 82 | 发送失败重试图标 + 重连条 SVG |
| 83 | 发送中时钟状态（WhatsApp 式） |
| 84 | 草稿标签色 + 群公告喇叭图标 |
| 85 | 移动端右滑回复（Telegram 式） |
| 86 | 引用回复条：色条 + SVG 图标 |
| 87 | 上传进度条图标化 |
| 88 | 安全区适配 + 列表按压 + 文件大小 |
| 89 | 发送按钮忙碌反馈（CometChat 式） |
| 90 | 修复文件上传：网关 /uploads 代理 MinIO + FormData |
| 91 | 智能体「杰尼助手」：自动好友 + LLM 日常陪聊 |
| 92 | 修复杰尼助手 LLM 加载 + 己方发消息未读红点 |
| 93 | Agent 即时派发 + 聊天区发消息后自动滚底 |
| 94 | 左右分栏独立滚动 + 打开会话默认最新消息 |
| 95 | 修复杰尼助手 DeepSeek 配置加载与 API 端点 |
| 96 | 智能体重命名为杰尼龟龟 + 杰尼龟头像 |
| 97 | 杰尼龟龟二次元萌系人设与对话语气 |
| 98 | 群聊邀请体系：好友建群、面对面建群、群号搜索 |
| 99 | 群管理员设置、面对面建群码刷新 |
| 100 | 踢人、转让群主、待处理入群邀请管理 |
| 101 | 群号 10 位 + 面对面 4 位码分离、公开搜索群聊 |
| 102 | Toast 提示自动消失（约 3 秒） |
| 103–105 | Electron 桌面版 + 排版模式（单窗口/独立聊天窗） |
| 106 | 桌面窗口位置记忆 + 托盘隐藏 |

完整记录见 `docs/phases/PHASE_*_DONE.md`。

## 开发循环

见 `.cursor/skills/squirtle-dev-cycle/SKILL.md`：需求 → 设计 → 开发 → 测试 → 文档。
