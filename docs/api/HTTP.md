# HTTP API v1

Base: `http://localhost:8080/api/v1`

## 通用响应
```json
{"code":0,"msg":"ok","data":{}}
```

## Auth
| Method | Path | Body | 说明 |
|--------|------|------|------|
| POST | /auth/register | username,password,nickname | 注册 |
| POST | /auth/login | username,password,device_id | 登录 |
| POST | /auth/refresh | refresh_token | 刷新 |
| POST | /auth/logout | - | 登出 |

## User
| GET | /users/me | 当前用户 |
| PUT | /users/me | nickname,avatar | 更新资料 |
| GET | /users/search?q= | 搜索用户 |

## Friend
| POST | /friends/request | to_user_id,message | 申请 |
| POST | /friends/request/:id/accept | 接受 |
| POST | /friends/request/:id/reject | 拒绝 |
| GET | /friends | 好友列表 |
| DELETE | /friends/:id | 删除好友 |
| GET | /friends/requests | 待处理申请 |

## Group
| POST | /groups | name,member_ids | 建群 |
| GET | /groups | 群列表 |
| GET | /groups/:id | 群详情（含 notice） |
| PUT | /groups/:id/notice | notice | 群主设置公告（≤200字） |
| POST | /groups/:id/members | user_ids | 邀请 |
| DELETE | /groups/:id/members/:uid | 踢人/退群 |

## Conversation
| GET | /conversations | 会话列表 |
| GET | /conversations/:id/messages?before_seq=&around_seq=&limit= | 历史消息（around_seq 定位附近） |
| GET | /conversations/:id/messages/search?q=&before_seq=&limit= | 会话内文本消息搜索 |

## Sync
| GET | /sync?since_seq=&limit= | 增量同步 |
| POST | /sync/read | conversation_id,read_seq | 已读 |

## File
| POST | /files/upload-token | filename,size,content_type | 上传凭证 |
| POST | /files/complete | file_id,parts | 完成上传 |
