package handler

import (
	"errors"
	"strconv"

	"squirtlechat/internal/middleware"
	"squirtlechat/internal/service"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

type FriendHandler struct {
	friend *service.FriendService
	auth   *service.AuthService
}

func NewFriendHandler(friend *service.FriendService, auth *service.AuthService) *FriendHandler {
	return &FriendHandler{friend: friend, auth: auth}
}

func (h *FriendHandler) Register(r *gin.RouterGroup) {
	g := r.Group("", middleware.Auth(h.auth))
	g.POST("/friends/request", h.request)
	g.POST("/friends/request/:id/accept", h.accept)
	g.POST("/friends/request/:id/reject", h.reject)
	g.PUT("/friends/:id/remark", h.setRemark)
	g.GET("/friends", h.list)
	g.DELETE("/friends/:id", h.delete)
	g.GET("/friends/requests", h.pending)
	g.POST("/groups", h.createGroup)
	g.POST("/groups/face-to-face/start", h.startFaceToFace)
	g.POST("/groups/face-to-face/join", h.joinFaceToFace)
	g.POST("/groups/join-by-no", h.joinByGroupNo)
	g.GET("/groups/by-no/:no", h.lookupGroupByNo)
	g.GET("/groups/discover", h.discoverGroups)
	g.GET("/groups/search-no", h.discoverGroups)
	g.GET("/groups/invitations", h.listGroupInvitations)
	g.GET("/groups/:id/invitations", h.listGroupPendingInvites)
	g.DELETE("/groups/:id/invitations/:inviteId", h.cancelGroupInvite)
	g.POST("/groups/:id/transfer", h.transferOwner)
	g.POST("/groups/invitations/:id/accept", h.acceptGroupInvitation)
	g.POST("/groups/invitations/:id/reject", h.rejectGroupInvitation)
	g.GET("/groups", h.listGroups)
	g.GET("/groups/search", h.searchGroups)
	g.GET("/groups/:id", h.getGroup)
	g.PUT("/groups/:id/notice", h.setGroupNotice)
	g.POST("/groups/:id/face-to-face/refresh", h.refreshFaceToFace)
	g.GET("/groups/:id/face-to-face", h.getFaceToFace)
	g.POST("/groups/:id/admins/:uid", h.setGroupAdmin)
	g.DELETE("/groups/:id/admins/:uid", h.unsetGroupAdmin)
	g.POST("/groups/:id/invites", h.inviteMembers)
	g.POST("/groups/:id/members", h.addMembers)
	g.DELETE("/groups/:id/members/:uid", h.removeMember)
}

func (h *FriendHandler) request(c *gin.Context) {
	var req struct {
		ToUserID string `json:"to_user_id" binding:"required"`
		Message  string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	toID, err := strconv.ParseInt(req.ToUserID, 10, 64)
	if err != nil || toID <= 0 {
		failParam(c, errors.New("用户 ID 无效"))
		return
	}
	if err := h.friend.Request(c.Request.Context(), middleware.UserID(c), toID, req.Message); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) accept(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.friend.Accept(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) reject(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.friend.Reject(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) delete(c *gin.Context) {
	fid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.friend.DeleteFriend(c.Request.Context(), middleware.UserID(c), fid); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) pending(c *gin.Context) {
	list, err := h.friend.ListPending(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"requests": list})
}

func (h *FriendHandler) list(c *gin.Context) {
	list, err := h.friend.ListFriendCards(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"friends": list})
}

func (h *FriendHandler) setRemark(c *gin.Context) {
	fid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || fid <= 0 {
		failParam(c, err)
		return
	}
	var req struct {
		Remark string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.friend.SetRemark(c.Request.Context(), middleware.UserID(c), fid, req.Remark); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) searchGroups(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.friend.SearchGroups(c.Request.Context(), middleware.UserID(c), q, limit)
	if err != nil {
		failParam(c, err)
		return
	}
	response.OK(c, gin.H{"groups": list})
}

func (h *FriendHandler) createGroup(c *gin.Context) {
	var req struct {
		Name             string   `json:"name" binding:"required"`
		InviteFriendIDs  []string `json:"invite_friend_ids"`
		MemberIDs        []string `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	ids := parseIDList(req.InviteFriendIDs)
	if len(ids) == 0 {
		ids = parseIDList(req.MemberIDs)
	}
	res, err := h.friend.CreateGroup(c.Request.Context(), middleware.UserID(c), req.Name, ids)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func parseIDList(ss []string) []int64 {
	out := make([]int64, 0, len(ss))
	for _, s := range ss {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		out = append(out, id)
	}
	return out
}

func (h *FriendHandler) startFaceToFace(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	res, err := h.friend.StartFaceToFace(c.Request.Context(), middleware.UserID(c), req.Code)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func (h *FriendHandler) joinFaceToFace(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	res, err := h.friend.JoinFaceToFace(c.Request.Context(), middleware.UserID(c), req.Code)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func (h *FriendHandler) joinByGroupNo(c *gin.Context) {
	var req struct {
		GroupNo string `json:"group_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.friend.RequestJoinByGroupNo(c.Request.Context(), middleware.UserID(c), req.GroupNo); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok", "message": "已发送入群邀请，请在通知中接受"})
}

func (h *FriendHandler) lookupGroupByNo(c *gin.Context) {
	res, err := h.friend.LookupGroupByNo(c.Request.Context(), c.Param("no"))
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func (h *FriendHandler) discoverGroups(c *gin.Context) {
	q := c.Query("q")
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.friend.DiscoverGroups(c.Request.Context(), middleware.UserID(c), q, limit)
	if err != nil {
		failParam(c, err)
		return
	}
	response.OK(c, gin.H{"groups": list})
}

func (h *FriendHandler) listGroupInvitations(c *gin.Context) {
	list, err := h.friend.ListGroupInvitations(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"invitations": list})
}

func (h *FriendHandler) acceptGroupInvitation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.friend.AcceptGroupInvitation(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) rejectGroupInvitation(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.friend.RejectGroupInvitation(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) inviteMembers(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		UserIDs []int64 `json:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	sent, err := h.friend.InviteGroupMembers(c.Request.Context(), gid, middleware.UserID(c), req.UserIDs)
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok", "invites_sent": sent})
}

func (h *FriendHandler) refreshFaceToFace(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.friend.RefreshFaceToFace(c.Request.Context(), gid, middleware.UserID(c))
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func (h *FriendHandler) getFaceToFace(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.friend.GetFaceToFaceSession(c.Request.Context(), gid, middleware.UserID(c))
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func (h *FriendHandler) setGroupAdmin(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err := h.friend.SetGroupAdmin(c.Request.Context(), gid, middleware.UserID(c), uid, true); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) unsetGroupAdmin(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err := h.friend.SetGroupAdmin(c.Request.Context(), gid, middleware.UserID(c), uid, false); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) listGroups(c *gin.Context) {
	list, err := h.friend.ListGroups(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		failInternal(c, err)
		return
	}
	response.OK(c, gin.H{"groups": list})
}

func (h *FriendHandler) getGroup(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := h.friend.GetGroup(c.Request.Context(), gid, middleware.UserID(c))
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, res)
}

func (h *FriendHandler) setGroupNotice(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Notice string `json:"notice"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.friend.SetGroupNotice(c.Request.Context(), gid, middleware.UserID(c), req.Notice); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok", "notice": req.Notice})
}

func (h *FriendHandler) addMembers(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		UserIDs []int64 `json:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	if err := h.friend.AddGroupMembers(c.Request.Context(), gid, middleware.UserID(c), req.UserIDs); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) removeMember(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	uid, _ := strconv.ParseInt(c.Param("uid"), 10, 64)
	me := middleware.UserID(c)
	if uid == 0 {
		uid = me
	}
	if err := h.friend.KickGroupMember(c.Request.Context(), gid, me, uid); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) listGroupPendingInvites(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	list, err := h.friend.ListGroupPendingInvites(c.Request.Context(), gid, middleware.UserID(c))
	if err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"invitations": list})
}

func (h *FriendHandler) cancelGroupInvite(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	inviteID, _ := strconv.ParseInt(c.Param("inviteId"), 10, 64)
	if err := h.friend.CancelGroupInvite(c.Request.Context(), gid, middleware.UserID(c), inviteID); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}

func (h *FriendHandler) transferOwner(c *gin.Context) {
	gid, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		NewOwnerID string `json:"new_owner_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failParam(c, err)
		return
	}
	newOwner, err := strconv.ParseInt(req.NewOwnerID, 10, 64)
	if err != nil || newOwner <= 0 {
		failParam(c, errors.New("新群主 ID 无效"))
		return
	}
	if err := h.friend.TransferGroupOwner(c.Request.Context(), gid, middleware.UserID(c), newOwner); err != nil {
		failConflict(c, err)
		return
	}
	response.OK(c, gin.H{"status": "ok"})
}
