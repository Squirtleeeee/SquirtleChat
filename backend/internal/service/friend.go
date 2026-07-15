package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"time"

	"squirtlechat/internal/model"
	"squirtlechat/internal/store"
	"squirtlechat/pkg/idgen"
)

const faceToFaceTTL = 10 * time.Minute

type FriendService struct {
	friend *store.FriendStore
	users  *store.UserStore
	idgen  *idgen.Generator
	msgs   *MessageService
}

func NewFriendService(friend *store.FriendStore, users *store.UserStore, gen *idgen.Generator) *FriendService {
	return &FriendService{friend: friend, users: users, idgen: gen}
}

func (s *FriendService) SetMessageService(msgs *MessageService) {
	s.msgs = msgs
}

func (s *FriendService) Request(ctx context.Context, from, to int64, message string) error {
	if from == to {
		return errBadRequest("不能添加自己为好友")
	}
	if _, err := s.users.GetByID(ctx, to); err != nil {
		return errBadRequest("用户不存在，请检查用户 ID")
	}
	ok, err := s.friend.AreFriends(ctx, from, to)
	if err != nil {
		return err
	}
	if ok {
		return errBadRequest("对方已经是您的好友")
	}
	pending, err := s.friend.HasPendingRequest(ctx, from, to)
	if err != nil {
		return err
	}
	if pending {
		return errBadRequest("好友申请已发送，请等待对方处理")
	}
	return s.friend.CreateRequest(ctx, from, to, message)
}

func (s *FriendService) Accept(ctx context.Context, reqID, userID int64) error {
	return s.friend.AcceptRequest(ctx, reqID, userID)
}

func (s *FriendService) Reject(ctx context.Context, reqID, userID int64) error {
	if err := s.friend.RejectRequest(ctx, reqID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("好友申请不存在或已处理")
		}
		return err
	}
	return nil
}

func (s *FriendService) DeleteFriend(ctx context.Context, userID, friendID int64) error {
	if err := s.friend.DeleteFriend(ctx, userID, friendID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("对方不是您的好友")
		}
		return err
	}
	return nil
}

func (s *FriendService) ListFriendCards(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	entries, err := s.friend.ListFriendEntries(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		u, err := s.users.GetByID(ctx, e.FriendID)
		if err != nil {
			continue
		}
		pub := u.ApplyPrivacy(false)
		out = append(out, map[string]interface{}{
			"id":       strconv.FormatInt(pub.ID, 10),
			"username": pub.Username,
			"nickname": pub.Nickname,
			"avatar":   pub.Avatar,
			"gender":   pub.Gender,
			"birthday": pub.Birthday,
			"remark":   e.Remark,
		})
	}
	return out, nil
}

func (s *FriendService) SetRemark(ctx context.Context, userID, friendID int64, remark string) error {
	if len([]rune(remark)) > 64 {
		return errBadRequest("备注过长")
	}
	if err := s.friend.SetRemark(ctx, userID, friendID, remark); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("对方不是您的好友")
		}
		return err
	}
	return nil
}

func (s *FriendService) ListFriends(ctx context.Context, userID int64) ([]int64, error) {
	return s.friend.ListFriends(ctx, userID)
}

func (s *FriendService) CreateGroup(ctx context.Context, ownerID int64, name string, inviteFriendIDs []int64) (map[string]interface{}, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errBadRequest("群名称不能为空")
	}
	gid := s.idgen.Next()
	convID, groupNo, err := s.friend.CreateGroup(ctx, gid, ownerID, name)
	if err != nil {
		return nil, err
	}
	sent, _ := s.sendFriendInvites(ctx, gid, ownerID, inviteFriendIDs, store.GroupInviteTypeFriend, "")
	return map[string]interface{}{
		"conversation_id": convID,
		"group_id":        strconv.FormatInt(gid, 10),
		"group_no":        groupNo,
		"invites_sent":    sent,
	}, nil
}

func (s *FriendService) StartFaceToFace(ctx context.Context, ownerID int64, code string) (map[string]interface{}, error) {
	code = strings.TrimSpace(code)
	if len(code) != 4 || !isAllDigits(code) {
		return nil, errBadRequest("请输入 4 位数字建群码")
	}
	taken, err := s.friend.IsActiveFaceCodeTaken(ctx, code)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errBadRequest("该建群码已被使用，请换一个")
	}
	gid := s.idgen.Next()
	convID, groupNo, err := s.friend.CreateGroup(ctx, gid, ownerID, "面对面群聊")
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(faceToFaceTTL)
	if err := s.friend.CreateFaceSessionWithCode(ctx, gid, ownerID, code, expiresAt); err != nil {
		return nil, errBadRequest("建群码设置失败，请换一个")
	}
	return map[string]interface{}{
		"conversation_id": convID,
		"group_id":        strconv.FormatInt(gid, 10),
		"group_no":        groupNo,
		"face_code":       code,
		"expires_at":      expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *FriendService) JoinFaceToFace(ctx context.Context, userID int64, code string) (map[string]interface{}, error) {
	code = strings.TrimSpace(code)
	if len(code) != 4 || !isAllDigits(code) {
		return nil, errBadRequest("请输入 4 位数字建群码")
	}
	sess, err := s.friend.GetActiveFaceSessionByCode(ctx, code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("建群码无效或已过期")
		}
		return nil, err
	}
	member, err := s.friend.IsGroupMember(ctx, sess.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if member {
		return nil, errBadRequest("您已在该群中")
	}
	if err := s.friend.AddGroupMember(ctx, sess.GroupID, userID); err != nil {
		return nil, err
	}
	s.announceMemberJoined(ctx, sess.GroupID, userID)
	g, _, err := s.friend.GetGroup(ctx, sess.GroupID)
	if err != nil {
		return map[string]interface{}{"status": "ok"}, nil
	}
	return map[string]interface{}{
		"status":          "ok",
		"group_id":        strconv.FormatInt(g.ID, 10),
		"conversation_id": g.ConversationID,
	}, nil
}

func (s *FriendService) RequestJoinByGroupNo(ctx context.Context, userID int64, groupNo string) error {
	groupNo = strings.TrimSpace(groupNo)
	if !isValidGroupNo(groupNo) {
		return errBadRequest("请输入正确的群号（约 10 位数字）")
	}
	g, err := s.friend.GetGroupByNo(ctx, groupNo)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("群不存在")
		}
		return err
	}
	return s.createJoinInvite(ctx, g.ID, userID, store.GroupInviteTypeGroupNo, "通过群号申请加入")
}

func (s *FriendService) LookupGroupByNo(ctx context.Context, groupNo string) (map[string]interface{}, error) {
	groupNo = strings.TrimSpace(groupNo)
	if groupNo == "" {
		return nil, errBadRequest("请输入群号")
	}
	g, err := s.friend.GetGroupByNo(ctx, groupNo)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("群不存在")
		}
		return nil, err
	}
	count, err := s.friend.CountGroupMembers(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":           strconv.FormatInt(g.ID, 10),
		"name":         g.Name,
		"group_no":     g.GroupNo,
		"member_count": count,
	}, nil
}

func (s *FriendService) SearchGroupsByNo(ctx context.Context, q string, limit int) ([]store.GroupPublicSummary, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, errBadRequest("请输入群名称或群号")
	}
	return s.friend.DiscoverGroups(ctx, q, limit)
}

func (s *FriendService) DiscoverGroups(ctx context.Context, userID int64, q string, limit int) ([]map[string]interface{}, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, errBadRequest("请输入群名称或群号")
	}
	list, err := s.friend.DiscoverGroups(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, g := range list {
		member := false
		if userID > 0 {
			member, _ = s.friend.IsGroupMember(ctx, g.ID, userID)
		}
		out = append(out, map[string]interface{}{
			"id":           strconv.FormatInt(g.ID, 10),
			"name":         g.Name,
			"group_no":     g.GroupNo,
			"member_count": g.MemberCount,
			"is_member":    member,
		})
	}
	return out, nil
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func isValidGroupNo(s string) bool {
	if len(s) < 8 || len(s) > 12 {
		return false
	}
	return isAllDigits(s)
}

func (s *FriendService) InviteGroupMembers(ctx context.Context, groupID, operatorID int64, userIDs []int64) (int, error) {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return 0, err
	}
	return s.sendFriendInvites(ctx, groupID, operatorID, userIDs, store.GroupInviteTypeFriend, "")
}

func (s *FriendService) ListGroupInvitations(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	rows, err := s.friend.ListPendingGroupInvites(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, inv := range rows {
		g, _, err := s.friend.GetGroup(ctx, inv.GroupID)
		if err != nil {
			continue
		}
		fromName := strconv.FormatInt(inv.FromUserID, 10)
		fromAvatar := ""
		if u, err := s.users.GetByID(ctx, inv.FromUserID); err == nil {
			pub := u.ApplyPrivacy(false)
			if pub.Nickname != "" {
				fromName = pub.Nickname
			} else {
				fromName = pub.Username
			}
			fromAvatar = pub.Avatar
		}
		out = append(out, map[string]interface{}{
			"id":           strconv.FormatInt(inv.ID, 10),
			"group_id":     strconv.FormatInt(inv.GroupID, 10),
			"group_name":   g.Name,
			"group_no":     g.GroupNo,
			"from_user_id": strconv.FormatInt(inv.FromUserID, 10),
			"from_name":    fromName,
			"from_avatar":  fromAvatar,
			"message":      inv.Message,
			"invite_type":  inv.InviteType,
		})
	}
	return out, nil
}

func (s *FriendService) AcceptGroupInvitation(ctx context.Context, inviteID, userID int64) error {
	groupID, err := s.friend.AcceptGroupInvitation(ctx, inviteID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("群邀请不存在或已处理")
		}
		return err
	}
	s.announceMemberJoined(ctx, groupID, userID)
	return nil
}

func (s *FriendService) RejectGroupInvitation(ctx context.Context, inviteID, userID int64) error {
	if err := s.friend.RejectGroupInvitation(ctx, inviteID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("群邀请不存在或已处理")
		}
		return err
	}
	return nil
}

func (s *FriendService) createJoinInvite(ctx context.Context, groupID, userID int64, inviteType int, message string) error {
	member, err := s.friend.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if member {
		return errBadRequest("您已在该群中")
	}
	pending, err := s.friend.HasPendingGroupInvite(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if pending {
		return errBadRequest("入群邀请已发送，请在通知中接受")
	}
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	_, err = s.friend.CreateGroupInvitation(ctx, groupID, g.OwnerID, userID, message, inviteType)
	return err
}

func (s *FriendService) sendFriendInvites(ctx context.Context, groupID, fromUserID int64, userIDs []int64, inviteType int, message string) (int, error) {
	seen := map[int64]bool{}
	sent := 0
	for _, uid := range userIDs {
		if uid <= 0 || uid == fromUserID || seen[uid] {
			continue
		}
		seen[uid] = true
		ok, err := s.friend.AreFriends(ctx, fromUserID, uid)
		if err != nil || !ok {
			continue
		}
		member, err := s.friend.IsGroupMember(ctx, groupID, uid)
		if err != nil || member {
			continue
		}
		pending, err := s.friend.HasPendingGroupInvite(ctx, groupID, uid)
		if err != nil || pending {
			continue
		}
		if _, err = s.friend.CreateGroupInvitation(ctx, groupID, fromUserID, uid, message, inviteType); err != nil {
			continue
		}
		sent++
	}
	return sent, nil
}

func (s *FriendService) SetGroupAdmin(ctx context.Context, groupID, ownerID, targetID int64, admin bool) error {
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	if g.OwnerID != ownerID {
		return errBadRequest("仅群主可设置管理员")
	}
	if targetID == ownerID {
		return errBadRequest("不能变更群主角色")
	}
	member, err := s.friend.IsGroupMember(ctx, groupID, targetID)
	if err != nil || !member {
		return errBadRequest("对方不在群中")
	}
	role := store.GroupRoleMember
	if admin {
		role = store.GroupRoleAdmin
	}
	return s.friend.SetMemberRole(ctx, groupID, targetID, role)
}

func (s *FriendService) RefreshFaceToFace(ctx context.Context, groupID, operatorID int64) (map[string]interface{}, error) {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(faceToFaceTTL)
	code, err := s.friend.RefreshFaceSession(ctx, groupID, operatorID, expiresAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"face_code":  code,
		"expires_at": expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *FriendService) GetFaceToFaceSession(ctx context.Context, groupID, userID int64) (map[string]interface{}, error) {
	member, err := s.friend.IsGroupMember(ctx, groupID, userID)
	if err != nil || !member {
		return nil, errBadRequest("您不在该群中")
	}
	sess, err := s.friend.GetActiveFaceSessionByGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("当前没有有效的面对面建群码")
		}
		return nil, err
	}
	return map[string]interface{}{
		"face_code":  sess.Code,
		"expires_at": sess.ExpiresAt.Format(time.RFC3339),
	}, nil
}

func (s *FriendService) requireGroupManager(ctx context.Context, groupID, operatorID int64) error {
	role, err := s.friend.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("您不在该群中")
		}
		return err
	}
	if role < store.GroupRoleAdmin {
		return errBadRequest("仅群主或管理员可邀请成员")
	}
	return nil
}

func (s *FriendService) ListGroups(ctx context.Context, userID int64) ([]store.GroupSummary, error) {
	return s.friend.ListUserGroups(ctx, userID)
}

func (s *FriendService) GetGroup(ctx context.Context, groupID, userID int64) (map[string]interface{}, error) {
	g, members, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("群不存在")
		}
		return nil, err
	}
	allowed := false
	for _, m := range members {
		if m == userID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errBadRequest("您不在该群中")
	}
	memberStrs := make([]string, len(members))
	for i, id := range members {
		memberStrs[i] = strconv.FormatInt(id, 10)
	}
	memberProfiles := make([]model.PublicProfile, 0, len(members))
	memberRoles := make(map[string]int)
	memberMuted := make(map[string]bool)
	memberNicknames := make(map[string]string)
	memberRemarks := make(map[string]string)
	roleRows, _ := s.friend.ListGroupMemberRoles(ctx, groupID)
	for _, r := range roleRows {
		uid := strconv.FormatInt(r.UserID, 10)
		memberRoles[uid] = r.Role
		if r.Muted {
			memberMuted[uid] = true
		}
		if n := strings.TrimSpace(r.Nickname); n != "" {
			memberNicknames[uid] = n
		}
	}
	if remaps, rerr := s.friend.ListGroupMemberRemarks(ctx, userID, groupID); rerr == nil {
		for tid, remark := range remaps {
			memberRemarks[strconv.FormatInt(tid, 10)] = remark
		}
	}
	for _, mid := range members {
		u, err := s.users.GetByID(ctx, mid)
		if err != nil {
			continue
		}
		memberProfiles = append(memberProfiles, u.ApplyPrivacy(userID == mid))
	}
	return map[string]interface{}{
		"id":               strconv.FormatInt(g.ID, 10),
		"name":             g.Name,
		"group_no":         g.GroupNo,
		"owner_id":         strconv.FormatInt(g.OwnerID, 10),
		"conversation_id":  g.ConversationID,
		"notice":           g.Notice,
		"welcome_text":     g.WelcomeText,
		"admin_only":       g.AdminOnly,
		"slow_mode_secs":   g.SlowModeSecs,
		"member_ids":       memberStrs,
		"member_roles":     memberRoles,
		"member_muted":     memberMuted,
		"member_nicknames": memberNicknames,
		"member_remarks":   memberRemarks,
		"members":          memberProfiles,
	}, nil
}

func (s *FriendService) SetGroupMemberRemark(ctx context.Context, groupID, viewerID, targetID int64, remark string) error {
	ok, err := s.friend.IsGroupMember(ctx, groupID, viewerID)
	if err != nil {
		return err
	}
	if !ok {
		return errBadRequest("您不在该群中")
	}
	tok, err := s.friend.IsGroupMember(ctx, groupID, targetID)
	if err != nil {
		return err
	}
	if !tok {
		return errBadRequest("目标用户不在群中")
	}
	if viewerID == targetID {
		return errBadRequest("不能备注自己，请改用群名片")
	}
	r := strings.TrimSpace(remark)
	if len([]rune(r)) > 32 {
		return errBadRequest("备注最多 32 字")
	}
	return s.friend.SetGroupMemberRemark(ctx, viewerID, groupID, targetID, r)
}

func (s *FriendService) SetMyGroupNickname(ctx context.Context, groupID, userID int64, nickname string) error {
	ok, err := s.friend.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return errBadRequest("您不在该群中")
	}
	nick := strings.TrimSpace(nickname)
	if len([]rune(nick)) > 32 {
		return errBadRequest("群名片最多 32 字")
	}
	return s.friend.SetMemberNickname(ctx, groupID, userID, nick)
}

func (s *FriendService) SetGroupAdminOnly(ctx context.Context, groupID, operatorID int64, adminOnly bool) error {
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	if g.OwnerID != operatorID {
		role, rerr := s.friend.GetMemberRole(ctx, groupID, operatorID)
		if rerr != nil || role < store.GroupRoleAdmin {
			return errBadRequest("仅群主或管理员可设置全员禁言")
		}
	}
	return s.friend.SetGroupAdminOnly(ctx, groupID, adminOnly)
}

func (s *FriendService) SetGroupSlowMode(ctx context.Context, groupID, operatorID int64, secs int) error {
	if secs < 0 || secs > 3600 {
		return errBadRequest("慢速间隔须在 0–3600 秒")
	}
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return err
	}
	return s.friend.SetGroupSlowMode(ctx, groupID, secs)
}

func (s *FriendService) SetGroupMemberMuted(ctx context.Context, groupID, operatorID, targetID int64, muted bool) error {
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	opRole, err := s.friend.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		return errBadRequest("您不在该群中")
	}
	if opRole < store.GroupRoleAdmin && g.OwnerID != operatorID {
		return errBadRequest("仅群主或管理员可禁言成员")
	}
	if targetID == g.OwnerID {
		return errBadRequest("不能禁言群主")
	}
	targetRole, err := s.friend.GetMemberRole(ctx, groupID, targetID)
	if err != nil {
		return errBadRequest("目标用户不在群中")
	}
	if opRole == store.GroupRoleAdmin && targetRole >= store.GroupRoleAdmin {
		return errBadRequest("管理员不能禁言其他管理员")
	}
	if operatorID == targetID {
		return errBadRequest("不能禁言自己")
	}
	return s.friend.SetMemberMuted(ctx, groupID, targetID, muted)
}

func (s *FriendService) SetGroupNotice(ctx context.Context, groupID, operatorID int64, notice string) error {
	g, members, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	if g.OwnerID != operatorID {
		return errBadRequest("仅群主可设置公告")
	}
	inGroup := false
	for _, m := range members {
		if m == operatorID {
			inGroup = true
			break
		}
	}
	if !inGroup {
		return errBadRequest("您不在该群中")
	}
	notice = strings.TrimSpace(notice)
	if len([]rune(notice)) > 200 {
		return errBadRequest("公告不能超过200字")
	}
	return s.friend.SetGroupNotice(ctx, groupID, notice)
}

func (s *FriendService) SetGroupWelcome(ctx context.Context, groupID, operatorID int64, welcome string) error {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return err
	}
	welcome = strings.TrimSpace(welcome)
	if len([]rune(welcome)) > 200 {
		return errBadRequest("欢迎语不能超过200字")
	}
	return s.friend.SetGroupWelcome(ctx, groupID, welcome)
}

func (s *FriendService) announceMemberJoined(ctx context.Context, groupID, userID int64) {
	if s.msgs == nil {
		return
	}
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return
	}
	name := strconv.FormatInt(userID, 10)
	if u, err := s.users.GetByID(ctx, userID); err == nil {
		pub := u.ApplyPrivacy(false)
		if pub.Nickname != "" {
			name = pub.Nickname
		} else if pub.Username != "" {
			name = pub.Username
		}
	}
	_ = s.msgs.PostSystemMessage(ctx, g.ConversationID, name+" 加入了群聊")
	if w := strings.TrimSpace(g.WelcomeText); w != "" {
		_ = s.msgs.PostSystemMessage(ctx, g.ConversationID, w)
	}
}

func (s *FriendService) AddGroupMembers(ctx context.Context, groupID, operatorID int64, userIDs []int64) error {
	sent, err := s.InviteGroupMembers(ctx, groupID, operatorID, userIDs)
	if err != nil {
		return err
	}
	if sent == 0 && len(userIDs) > 0 {
		return errBadRequest("没有可邀请的好友（需为好友且未在群中）")
	}
	return nil
}

func (s *FriendService) KickGroupMember(ctx context.Context, groupID, operatorID, targetID int64) error {
	if operatorID == targetID {
		return s.LeaveGroup(ctx, groupID, targetID)
	}
	opRole, err := s.friend.GetMemberRole(ctx, groupID, operatorID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("您不在该群中")
		}
		return err
	}
	if opRole < store.GroupRoleAdmin {
		return errBadRequest("仅群主或管理员可移出成员")
	}
	targetRole, err := s.friend.GetMemberRole(ctx, groupID, targetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("对方不在该群中")
		}
		return err
	}
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	if targetID == g.OwnerID {
		return errBadRequest("不能移出群主")
	}
	if opRole == store.GroupRoleAdmin && targetRole >= store.GroupRoleAdmin {
		return errBadRequest("管理员不能移出其他管理员")
	}
	if err := s.friend.RemoveGroupMember(ctx, groupID, targetID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("对方不在该群中")
		}
		return err
	}
	return nil
}

func (s *FriendService) TransferGroupOwner(ctx context.Context, groupID, ownerID, newOwnerID int64) error {
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	if g.OwnerID != ownerID {
		return errBadRequest("仅群主可转让群主")
	}
	if newOwnerID == ownerID {
		return errBadRequest("不能转让给自己")
	}
	member, err := s.friend.IsGroupMember(ctx, groupID, newOwnerID)
	if err != nil || !member {
		return errBadRequest("新群主须在群内")
	}
	if err := s.friend.TransferGroupOwner(ctx, groupID, newOwnerID); err != nil {
		return err
	}
	return nil
}

func (s *FriendService) ListGroupPendingInvites(ctx context.Context, groupID, operatorID int64) ([]map[string]interface{}, error) {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return nil, err
	}
	rows, err := s.friend.ListPendingGroupInvitesByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, inv := range rows {
		toName := strconv.FormatInt(inv.ToUserID, 10)
		toAvatar := ""
		if u, err := s.users.GetByID(ctx, inv.ToUserID); err == nil {
			pub := u.ApplyPrivacy(false)
			if pub.Nickname != "" {
				toName = pub.Nickname
			} else {
				toName = pub.Username
			}
			toAvatar = pub.Avatar
		}
		out = append(out, map[string]interface{}{
			"id":          strconv.FormatInt(inv.ID, 10),
			"to_user_id":  strconv.FormatInt(inv.ToUserID, 10),
			"to_name":     toName,
			"to_avatar":   toAvatar,
			"message":     inv.Message,
			"invite_type": inv.InviteType,
			"created_at":  inv.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *FriendService) CancelGroupInvite(ctx context.Context, groupID, operatorID, inviteID int64) error {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return err
	}
	if err := s.friend.CancelGroupInvitation(ctx, inviteID, groupID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("邀请不存在或已处理")
		}
		return err
	}
	return nil
}

func (s *FriendService) LeaveGroup(ctx context.Context, groupID, userID int64) error {
	g, _, err := s.friend.GetGroup(ctx, groupID)
	if err != nil {
		return errBadRequest("群不存在")
	}
	if g.OwnerID == userID {
		return errBadRequest("群主请先转让群主后再退出")
	}
	if err := s.friend.RemoveGroupMember(ctx, groupID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("您不在该群中")
		}
		return err
	}
	return nil
}

func (s *FriendService) SearchGroups(ctx context.Context, userID int64, q string, limit int) ([]store.GroupSummary, error) {
	if q == "" {
		return nil, errBadRequest("请输入搜索关键词")
	}
	return s.friend.SearchGroups(ctx, userID, q, limit)
}

func (s *FriendService) ListPending(ctx context.Context, userID int64) ([]map[string]interface{}, error) {
	rows, err := s.friend.ListPendingRequests(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		fromUser, _ := s.users.GetByID(ctx, r.FromUserID)
		displayName := strconv.FormatInt(r.FromUserID, 10)
		avatar := ""
		if fromUser != nil {
			pub := fromUser.ApplyPrivacy(false)
			if pub.Nickname != "" {
				displayName = pub.Nickname
			} else {
				displayName = pub.Username
			}
			avatar = pub.Avatar
		}
		out = append(out, map[string]interface{}{
			"id":           strconv.FormatInt(r.ID, 10),
			"from_user_id": strconv.FormatInt(r.FromUserID, 10),
			"display_name": displayName,
			"avatar":       avatar,
			"message":      r.Message,
		})
	}
	return out, nil
}

func randomInviteCode(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	b := make([]byte, n)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = alphabet[v.Int64()]
	}
	return string(b), nil
}

func (s *FriendService) CreateInviteLink(ctx context.Context, groupID, operatorID int64, maxUses, expiresHours int) (map[string]interface{}, error) {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return nil, err
	}
	if maxUses < 0 || maxUses > 10000 {
		return nil, errBadRequest("使用次数无效")
	}
	if expiresHours < 0 || expiresHours > 24*90 {
		return nil, errBadRequest("有效期无效")
	}
	var exp *time.Time
	if expiresHours > 0 {
		t := time.Now().Add(time.Duration(expiresHours) * time.Hour)
		exp = &t
	}
	var code string
	var linkID int64
	for attempt := 0; attempt < 8; attempt++ {
		c, err := randomInviteCode(8)
		if err != nil {
			return nil, err
		}
		linkID = s.idgen.Next()
		link := &store.GroupInviteLink{
			ID:        linkID,
			GroupID:   groupID,
			Code:      c,
			CreatedBy: operatorID,
			MaxUses:   maxUses,
			ExpiresAt: exp,
		}
		if err := s.friend.CreateInviteLink(ctx, link); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				continue
			}
			return nil, err
		}
		code = c
		break
	}
	if code == "" {
		return nil, errBadRequest("生成邀请码失败，请重试")
	}
	out := map[string]interface{}{
		"id":        strconv.FormatInt(linkID, 10),
		"group_id":  strconv.FormatInt(groupID, 10),
		"code":      code,
		"max_uses":  maxUses,
		"use_count": 0,
		"revoked":   false,
	}
	if exp != nil {
		out["expires_at"] = exp.Format(time.RFC3339)
	}
	return out, nil
}

func (s *FriendService) ListInviteLinks(ctx context.Context, groupID, operatorID int64) ([]map[string]interface{}, error) {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return nil, err
	}
	rows, err := s.friend.ListInviteLinks(ctx, groupID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]map[string]interface{}, 0, len(rows))
	for _, l := range rows {
		item := map[string]interface{}{
			"id":         strconv.FormatInt(l.ID, 10),
			"group_id":   strconv.FormatInt(l.GroupID, 10),
			"code":       l.Code,
			"max_uses":   l.MaxUses,
			"use_count":  l.UseCount,
			"revoked":    l.Revoked,
			"created_at": l.CreatedAt.Format(time.RFC3339),
		}
		if l.ExpiresAt != nil {
			item["expires_at"] = l.ExpiresAt.Format(time.RFC3339)
			item["expired"] = !l.ExpiresAt.After(now)
		} else {
			item["expired"] = false
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *FriendService) RevokeInviteLink(ctx context.Context, groupID, linkID, operatorID int64) error {
	if err := s.requireGroupManager(ctx, groupID, operatorID); err != nil {
		return err
	}
	if err := s.friend.RevokeInviteLink(ctx, groupID, linkID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("邀请链接不存在或已撤销")
		}
		return err
	}
	return nil
}

func (s *FriendService) PreviewInviteLink(ctx context.Context, code string, userID int64) (map[string]interface{}, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errBadRequest("请输入邀请码")
	}
	link, err := s.friend.GetInviteLinkByCode(ctx, code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("邀请链接无效")
		}
		return nil, err
	}
	g, _, err := s.friend.GetGroup(ctx, link.GroupID)
	if err != nil {
		return nil, errBadRequest("群不存在")
	}
	count, _ := s.friend.CountGroupMembers(ctx, link.GroupID)
	member := false
	if userID > 0 {
		member, _ = s.friend.IsGroupMember(ctx, link.GroupID, userID)
	}
	expired := link.Revoked || (link.ExpiresAt != nil && !link.ExpiresAt.After(time.Now())) ||
		(link.MaxUses > 0 && link.UseCount >= link.MaxUses)
	out := map[string]interface{}{
		"code":         link.Code,
		"group_id":     strconv.FormatInt(g.ID, 10),
		"group_name":   g.Name,
		"group_no":     g.GroupNo,
		"member_count": count,
		"is_member":    member,
		"usable":       !expired,
	}
	if link.ExpiresAt != nil {
		out["expires_at"] = link.ExpiresAt.Format(time.RFC3339)
	}
	return out, nil
}

func (s *FriendService) JoinViaInviteLink(ctx context.Context, code string, userID int64) (map[string]interface{}, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errBadRequest("请输入邀请码")
	}
	existing, err := s.friend.GetInviteLinkByCode(ctx, code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("邀请链接无效")
		}
		return nil, err
	}
	ok, err := s.friend.IsGroupMember(ctx, existing.GroupID, userID)
	if err != nil {
		return nil, err
	}
	if ok {
		g, _, _ := s.friend.GetGroup(ctx, existing.GroupID)
		return map[string]interface{}{
			"status":          "already_member",
			"group_id":        strconv.FormatInt(existing.GroupID, 10),
			"conversation_id": g.ConversationID,
			"name":            g.Name,
		}, nil
	}
	link, err := s.friend.ConsumeInviteLink(ctx, code)
	if err != nil {
		switch err.Error() {
		case "link revoked":
			return nil, errBadRequest("邀请链接已撤销")
		case "link expired":
			return nil, errBadRequest("邀请链接已过期")
		case "link exhausted":
			return nil, errBadRequest("邀请链接已达使用上限")
		}
		if errors.Is(err, store.ErrNotFound) {
			return nil, errBadRequest("邀请链接无效")
		}
		return nil, err
	}
	if err := s.friend.AddGroupMember(ctx, link.GroupID, userID); err != nil {
		return nil, err
	}
	s.announceMemberJoined(ctx, link.GroupID, userID)
	g, _, err := s.friend.GetGroup(ctx, link.GroupID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":          "joined",
		"group_id":        strconv.FormatInt(g.ID, 10),
		"conversation_id": g.ConversationID,
		"name":            g.Name,
	}, nil
}

type errBadRequest string

func (e errBadRequest) Error() string { return string(e) }
