package service

import (
	"context"
	"errors"
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
}

func NewFriendService(friend *store.FriendStore, users *store.UserStore, gen *idgen.Generator) *FriendService {
	return &FriendService{friend: friend, users: users, idgen: gen}
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
	if err := s.friend.AcceptGroupInvitation(ctx, inviteID, userID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return errBadRequest("群邀请不存在或已处理")
		}
		return err
	}
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
	roleRows, _ := s.friend.ListGroupMemberRoles(ctx, groupID)
	for _, r := range roleRows {
		memberRoles[strconv.FormatInt(r.UserID, 10)] = r.Role
	}
	for _, mid := range members {
		u, err := s.users.GetByID(ctx, mid)
		if err != nil {
			continue
		}
		memberProfiles = append(memberProfiles, u.ApplyPrivacy(userID == mid))
	}
	return map[string]interface{}{
		"id":              strconv.FormatInt(g.ID, 10),
		"name":            g.Name,
		"group_no":        g.GroupNo,
		"owner_id":        strconv.FormatInt(g.OwnerID, 10),
		"conversation_id": g.ConversationID,
		"notice":          g.Notice,
		"member_ids":      memberStrs,
		"member_roles":    memberRoles,
		"members":         memberProfiles,
	}, nil
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

type errBadRequest string

func (e errBadRequest) Error() string { return string(e) }
