# Phase 59 完成

## 需求分析
- 需要引用回复：对某条消息点「回复」，发送时带上被引内容摘要
- 现有消息模型无 `reply_to` 字段；本阶段不改库表/API
- 侧栏预览、复制、多选转发不应露出协议垃圾字符

## 设计计划
- 前端协议：`⟦sq-reply⟧{n,p,c?}⟦/sq-reply⟧\n正文` 嵌入 `content`
- `utils/reply.ts`：encode/decode；`previewMessage` 剥离协议
- UI：气泡「回复」、composer 引用条、气泡内引用块；Esc / 切会话取消

## 交付
- `frontend/src/utils/reply.ts` — 协议编解码
- `frontend/src/utils/format.ts` — 预览剥离引用头
- `frontend/src/stores/chat.ts` — `sendText(text, reply?)`
- `frontend/src/views/ChatView.vue` — 回复操作、引用条、气泡引用渲染

## 测试
```powershell
cd frontend; npm run build
```
通过（vue-tsc + vite build）

## 验证
- 对文本/图片/文件消息点「回复」，输入框上方出现引用条
- 发送后气泡顶部显示被引昵称与摘要，正文正常
- 侧栏预览只显示正文，不含 `⟦sq-reply⟧`
- 复制 / 双击复制 / 多选转发文本不含协议头
- Esc 或 × 取消引用；切换会话清除引用目标

## 下阶段
Phase 60：待用户继续（本阶段后已暂停）
