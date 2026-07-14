package apperr

import "strings"

// ToUserMessage converts internal/backend errors to user-facing Chinese.
func ToUserMessage(code int, err error) string {
	if err != nil {
		raw := err.Error()
		if hasChinese(raw) {
			return raw
		}
		if msg := translateRaw(raw); msg != "" {
			return msg
		}
	}
	return CodeMessage(code)
}

func hasChinese(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func CodeMessage(code int) string {
	switch code {
	case ErrInvalidParam:
		return "请求参数不正确，请检查后重试"
	case ErrUnauthorized:
		return "账号或密码错误，请重新输入"
	case ErrForbidden:
		return "没有权限执行此操作"
	case ErrNotFound:
		return "请求的资源不存在"
	case ErrConflict:
		return "操作冲突，请刷新后重试"
	case ErrInternal:
		return "服务器内部错误，请稍后重试"
	default:
		return "操作失败，请稍后重试"
	}
}

func translateRaw(raw string) string {
	lower := strings.ToLower(raw)

	switch {
	case strings.Contains(raw, "Duplicate entry") && strings.Contains(raw, "username"):
		return "用户名已被注册，请更换用户名或直接登录"
	case strings.Contains(raw, "Duplicate entry"):
		return "数据已存在，请勿重复提交"
	case strings.Contains(lower, "username and password required"):
		return "用户名和密码不能为空"
	case strings.Contains(lower, "invalid credentials"):
		return "账号或密码错误，请重新输入"
	case strings.Contains(lower, "cannot add self"):
		return "不能添加自己为好友"
	case strings.Contains(lower, "user not found"), strings.Contains(lower, "not found"):
		return "用户不存在，请检查用户 ID"
	case strings.Contains(lower, "not friends"):
		return "你们还不是好友，请先添加好友"
	case strings.Contains(lower, "invalid message"):
		return "消息内容无效"
	case strings.Contains(lower, "to_user_id required"):
		return "请指定聊天对象"
	case strings.Contains(lower, "conversation_id required"):
		return "会话不存在"
	case strings.Contains(lower, "no recipients"):
		return "找不到消息接收人"
	case strings.Contains(lower, "bad conversation id"):
		return "会话 ID 无效"
	case strings.Contains(lower, "token required"):
		return "请先登录后再连接"
	case strings.Contains(lower, "unauthorized"):
		return "未登录或登录已过期，请重新登录"
	case strings.Contains(lower, "invalid token"):
		return "登录状态无效，请重新登录"
	case strings.Contains(lower, "missing form body"), strings.Contains(lower, "no such file"):
		return "请选择要上传的文件"
	case strings.Contains(lower, "file too large"), strings.Contains(lower, "request entity too large"):
		return "文件过大，请上传 20MB 以内的文件"
	case strings.Contains(lower, "connection refused"), strings.Contains(lower, "dial tcp"):
		return "服务暂时不可用，请稍后重试"
	case strings.Contains(lower, "no rows in result set"), strings.Contains(raw, "sql: no rows"):
		return "记录不存在或已处理，请刷新后重试"
	case strings.Contains(lower, "access denied"):
		return "数据库连接失败，请联系管理员"
	case strings.Contains(lower, "syntax error") && strings.Contains(lower, "groups"):
		return "群聊数据查询失败，请重启后端服务"
	}

	if strings.Contains(lower, "field validation") {
		switch {
		case strings.Contains(lower, "username"):
			return "请输入用户名"
		case strings.Contains(lower, "password"):
			return "请输入密码"
		case strings.Contains(lower, "nickname"):
			return "请输入昵称"
		case strings.Contains(lower, "to_user_id"):
			return "请输入好友用户 ID"
		case strings.Contains(lower, "name"):
			return "请输入群名称"
		case strings.Contains(lower, "conversation_id"):
			return "请指定会话"
		case strings.Contains(lower, "read_seq"):
			return "请指定已读位置"
		}
		return "请填写完整的必填信息"
	}

	if strings.Contains(lower, "json:") || strings.Contains(lower, "unmarshal") {
		return "请求数据格式错误"
	}
	return ""
}
