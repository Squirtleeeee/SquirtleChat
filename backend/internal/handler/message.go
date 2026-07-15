package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/model"
	"squirtlechat/internal/service"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	msg   *service.MessageService
	auth  *service.AuthService
	agent *service.AgentService
}

func NewMessageHandler(msg *service.MessageService, auth *service.AuthService, agent *service.AgentService) *MessageHandler {
	return &MessageHandler{msg: msg, auth: auth, agent: agent}
}

func (h *MessageHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.GET("/conversations", h.listConversations)
	g.GET("/messages/search", h.searchGlobal)
	g.GET("/mentions", h.listMentions)
	g.GET("/conversations/:id/messages/search", h.search)
	g.GET("/conversations/:id/messages/by-hashtag", h.searchByHashtag)
	g.GET("/conversations/:id/media", h.listMedia)
	g.GET("/conversations/:id/hashtags", h.listHashtags)
	g.GET("/conversations/:id/messages", h.list)
	g.GET("/conversations/:id/export", h.export)
	g.POST("/conversations/:id/messages/:msg_id/recall", h.recall)
	g.POST("/conversations/:id/messages/:msg_id/edit", h.edit)
	g.POST("/conversations/:id/messages/:msg_id/translate", h.translate)
	g.POST("/conversations/:id/messages/:msg_id/reactions", h.react)
	g.GET("/conversations/:id/pins", h.listPins)
	g.POST("/conversations/:id/pins", h.pin)
	g.DELETE("/conversations/:id/pins/:msg_id", h.unpin)
	g.GET("/conversations/:id/bookmarks", h.listBookmarks)
	g.POST("/conversations/:id/bookmarks", h.addBookmark)
	g.DELETE("/conversations/:id/bookmarks/:bookmark_id", h.deleteBookmark)
	g.GET("/stars", h.listStars)
	g.POST("/conversations/:id/messages/:msg_id/star", h.toggleStar)
	g.POST("/conversations/:id/messages/:msg_id/poll/vote", h.votePoll)
	g.GET("/scheduled-messages", h.listScheduled)
	g.POST("/scheduled-messages", h.createScheduled)
	g.DELETE("/scheduled-messages/:id", h.cancelScheduled)
	g.GET("/reminders", h.listReminders)
	g.POST("/conversations/:id/messages/:msg_id/remind", h.createReminder)
	g.DELETE("/reminders/:id", h.cancelReminder)
}

func (h *MessageHandler) listConversations(c *gin.Context) {
	list, err := h.msg.ListConversations(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"conversations": list})
}

func (h *MessageHandler) searchGlobal(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.SearchGlobal(c.Request.Context(), middleware.UserID(c), q, limit)
	if err != nil {
		if err.Error() == "请输入搜索关键词" || err.Error() == "搜索关键词过长" {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	response.OK(c, gin.H{"messages": msgs, "q": q})
}

func (h *MessageHandler) listMentions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	uid := middleware.UserID(c)
	patterns := []string{"@所有人"}
	if u, err := h.auth.GetProfile(c.Request.Context(), uid); err == nil && u != nil {
		if u.Nickname != "" {
			patterns = append(patterns, "@"+u.Nickname)
		}
		if u.Username != "" {
			patterns = append(patterns, "@"+u.Username)
		}
	}
	msgs, err := h.msg.ListUnreadMentions(c.Request.Context(), uid, patterns, limit)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"messages": msgs})
}

func (h *MessageHandler) list(c *gin.Context) {
	convID := c.Param("id")
	before, _ := strconv.ParseInt(c.Query("before_seq"), 10, 64)
	around, _ := strconv.ParseInt(c.Query("around_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.ListMessages(c.Request.Context(), middleware.UserID(c), convID, before, around, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	reactions, _ := h.msg.ReactionsForMessages(c.Request.Context(), middleware.UserID(c), msgs)
	if reactions == nil {
		reactions = map[string][]store.ReactionSummary{}
	}
	stars, _ := h.msg.StarredFlags(c.Request.Context(), middleware.UserID(c), msgs)
	if stars == nil {
		stars = map[string]bool{}
	}
	polls, _ := h.msg.PollsForMessages(c.Request.Context(), middleware.UserID(c), msgs)
	if polls == nil {
		polls = map[string]*store.PollResult{}
	}
	response.OK(c, gin.H{"messages": msgs, "reactions": reactions, "stars": stars, "polls": polls})
}

func (h *MessageHandler) export(c *gin.Context) {
	convID := c.Param("id")
	text, err := h.msg.ExportTranscript(c.Request.Context(), middleware.UserID(c), convID)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="chat-`+convID+`.txt"`)
	c.String(200, text)
}

func (h *MessageHandler) search(c *gin.Context) {
	convID := c.Param("id")
	q := c.Query("q")
	before, _ := strconv.ParseInt(c.Query("before_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.SearchMessages(c.Request.Context(), middleware.UserID(c), convID, q, before, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "请输入搜索关键词" || err.Error() == "搜索关键词过长" {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	if msgs == nil {
		msgs = []*model.Message{}
	}
	response.OK(c, gin.H{"messages": msgs, "q": q})
}

func (h *MessageHandler) recall(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	msg, err := h.msg.RecallMessage(c.Request.Context(), middleware.UserID(c), convID, msgID)
	if err != nil {
		if err.Error() == "无权操作该会话" || err.Error() == "只能撤回自己的消息" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err == store.ErrNotFound || err.Error() == "消息已撤回" {
			response.Fail(c, 404, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"message": msg})
}

func (h *MessageHandler) translate(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	var req struct {
		TargetLang string `json:"target_lang"`
	}
	_ = c.ShouldBindJSON(&req)
	msg, err := h.msg.GetMessageForMember(c.Request.Context(), middleware.UserID(c), convID, msgID)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	if msg.MsgType != model.MsgTypeText {
		response.Fail(c, 400, "仅支持翻译文本消息")
		return
	}
	text := strings.TrimSpace(msg.Content)
	// strip reply envelope if present: {"reply":...,"text":"..."}
	if strings.HasPrefix(text, "{") {
		var envelope struct {
			Text string `json:"text"`
		}
		if json.Unmarshal([]byte(text), &envelope) == nil && strings.TrimSpace(envelope.Text) != "" {
			text = strings.TrimSpace(envelope.Text)
		}
	}
	if h.agent == nil {
		response.Fail(c, 503, "翻译服务不可用")
		return
	}
	translated, lang, err := h.agent.Translate(c.Request.Context(), text, req.TargetLang)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{
		"msg_id":         strconv.FormatInt(msg.ID, 10),
		"target_lang":    lang,
		"translation":    translated,
		"source_preview": truncateRunes(text, 80),
	})
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (h *MessageHandler) edit(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	msg, err := h.msg.EditMessage(c.Request.Context(), middleware.UserID(c), convID, msgID, req.Content)
	if err != nil {
		if err.Error() == "无权操作该会话" || err.Error() == "只能编辑自己的消息" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "内容不能为空" || err.Error() == "内容过长" ||
			err.Error() == "仅支持编辑文本消息" || err.Error() == "超过15分钟无法编辑" {
			response.Fail(c, 400, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"message": msg})
}

func (h *MessageHandler) react(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	var req struct {
		Emoji string `json:"emoji" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	summaries, err := h.msg.ToggleReaction(c.Request.Context(), middleware.UserID(c), convID, msgID, req.Emoji)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "不支持的表情" || err.Error() == "系统消息不能回应" {
			response.Fail(c, 400, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{
		"msg_id":          strconv.FormatInt(msgID, 10),
		"conversation_id": convID,
		"reactions":       summaries,
	})
}

func (h *MessageHandler) listPins(c *gin.Context) {
	convID := c.Param("id")
	pins, err := h.msg.ListPins(c.Request.Context(), middleware.UserID(c), convID)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"pins": pins})
}

func (h *MessageHandler) pin(c *gin.Context) {
	convID := c.Param("id")
	var req struct {
		MsgID string `json:"msg_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	msgID, err := strconv.ParseInt(req.MsgID, 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	pins, err := h.msg.PinMessage(c.Request.Context(), middleware.UserID(c), convID, msgID)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "该消息不可置顶" || strings.Contains(err.Error(), "最多置顶") {
			response.Fail(c, 400, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"pins": pins, "msg_id": req.MsgID, "pinned": true})
}

func (h *MessageHandler) unpin(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	pins, err := h.msg.UnpinMessage(c.Request.Context(), middleware.UserID(c), convID, msgID)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "未置顶或不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"pins": pins, "msg_id": strconv.FormatInt(msgID, 10), "pinned": false})
}

func (h *MessageHandler) listBookmarks(c *gin.Context) {
	convID := c.Param("id")
	list, err := h.msg.ListBookmarks(c.Request.Context(), middleware.UserID(c), convID)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"bookmarks": list})
}

func (h *MessageHandler) addBookmark(c *gin.Context) {
	convID := c.Param("id")
	var req struct {
		Title string `json:"title" binding:"required"`
		URL   string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	b, err := h.msg.AddBookmark(c.Request.Context(), middleware.UserID(c), convID, req.Title, req.URL)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "标题和链接不能为空" || err.Error() == "标题过长" || err.Error() == "链接过长" ||
			err.Error() == "仅支持 http/https 链接" || strings.Contains(err.Error(), "最多") {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"bookmark": b})
}

func (h *MessageHandler) deleteBookmark(c *gin.Context) {
	convID := c.Param("id")
	bid, err := strconv.ParseInt(c.Param("bookmark_id"), 10, 64)
	if err != nil || bid <= 0 {
		response.Fail(c, 400, "书签 ID 无效")
		return
	}
	if err := h.msg.DeleteBookmark(c.Request.Context(), middleware.UserID(c), convID, bid); err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "书签不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *MessageHandler) listStars(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.msg.ListStarred(c.Request.Context(), middleware.UserID(c), limit)
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"stars": list})
}

func (h *MessageHandler) toggleStar(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	starred, err := h.msg.ToggleStar(c.Request.Context(), middleware.UserID(c), convID, msgID)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "系统消息不可收藏" {
			response.Fail(c, 400, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{
		"msg_id":  strconv.FormatInt(msgID, 10),
		"starred": starred,
	})
}

func (h *MessageHandler) votePoll(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	var req struct {
		OptionID string `json:"option_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	res, err := h.msg.VotePoll(c.Request.Context(), middleware.UserID(c), convID, msgID, req.OptionID)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "请选择选项" || err.Error() == "该消息不是投票" ||
			err.Error() == "投票内容无效" || err.Error() == "选项不存在" {
			response.Fail(c, 400, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"poll": res})
}

func (h *MessageHandler) listScheduled(c *gin.Context) {
	list, err := h.msg.ListScheduled(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"items": list})
}

func (h *MessageHandler) createScheduled(c *gin.Context) {
	var req struct {
		ConversationID   string `json:"conversation_id" binding:"required"`
		ConversationType int8   `json:"conversation_type"`
		ToUserID         string `json:"to_user_id"`
		Content          string `json:"content" binding:"required"`
		SendAt           string `json:"send_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	sendAt, err := time.Parse(time.RFC3339, req.SendAt)
	if err != nil {
		response.Fail(c, 400, "send_at 需为 RFC3339")
		return
	}
	var toUID int64
	if req.ToUserID != "" {
		toUID, err = strconv.ParseInt(req.ToUserID, 10, 64)
		if err != nil {
			response.Fail(c, 400, "to_user_id 无效")
			return
		}
	}
	item, err := h.msg.ScheduleMessage(
		c.Request.Context(), middleware.UserID(c), req.ConversationID, req.ConversationType, toUID, req.Content, sendAt,
	)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "内容不能为空" || err.Error() == "内容过长" ||
			err.Error() == "发送时间至少在 30 秒之后" || err.Error() == "最多提前 30 天定时" {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"item": item})
}

func (h *MessageHandler) cancelScheduled(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, 400, "ID 无效")
		return
	}
	if err := h.msg.CancelScheduled(c.Request.Context(), middleware.UserID(c), id); err != nil {
		if err == store.ErrNotFound {
			response.Fail(c, 404, "定时消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *MessageHandler) listReminders(c *gin.Context) {
	list, err := h.msg.ListReminders(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"items": list})
}

func (h *MessageHandler) createReminder(c *gin.Context) {
	convID := c.Param("id")
	msgID, err := strconv.ParseInt(c.Param("msg_id"), 10, 64)
	if err != nil || msgID <= 0 {
		response.Fail(c, 400, "消息 ID 无效")
		return
	}
	var req struct {
		RemindAt string `json:"remind_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	remindAt, err := time.Parse(time.RFC3339, req.RemindAt)
	if err != nil {
		response.Fail(c, 400, "remind_at 需为 RFC3339")
		return
	}
	item, err := h.msg.CreateReminder(c.Request.Context(), middleware.UserID(c), convID, msgID, remindAt)
	if err != nil {
		if err.Error() == "无权操作该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "系统消息不可提醒" ||
			err.Error() == "提醒时间至少在 30 秒之后" ||
			err.Error() == "最多提前 30 天提醒" {
			response.Fail(c, 400, err.Error())
			return
		}
		if err == store.ErrNotFound {
			response.Fail(c, 404, "消息不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"item": item})
}

func (h *MessageHandler) cancelReminder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Fail(c, 400, "ID 无效")
		return
	}
	if err := h.msg.CancelReminder(c.Request.Context(), middleware.UserID(c), id); err != nil {
		if err == store.ErrNotFound {
			response.Fail(c, 404, "提醒不存在")
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *MessageHandler) listHashtags(c *gin.Context) {
	convID := c.Param("id")
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.msg.ListHashtags(c.Request.Context(), middleware.UserID(c), convID, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"hashtags": list})
}

func (h *MessageHandler) searchByHashtag(c *gin.Context) {
	convID := c.Param("id")
	tag := c.Query("tag")
	before, _ := strconv.ParseInt(c.Query("before_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.SearchByHashtag(c.Request.Context(), middleware.UserID(c), convID, tag, before, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		if err.Error() == "请输入话题标签" || err.Error() == "标签过长" {
			response.Fail(c, 400, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"messages": msgs, "tag": strings.TrimPrefix(strings.TrimSpace(tag), "#")})
}

func (h *MessageHandler) listMedia(c *gin.Context) {
	convID := c.Param("id")
	kind := c.Query("kind")
	before, _ := strconv.ParseInt(c.Query("before_seq"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	msgs, err := h.msg.ListMedia(c.Request.Context(), middleware.UserID(c), convID, kind, before, limit)
	if err != nil {
		if err.Error() == "无权查看该会话" {
			response.Fail(c, 403, err.Error())
			return
		}
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"messages": msgs, "kind": kind})
}
