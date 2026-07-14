# 数据库 Schema

## users
| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT PK | Snowflake |
| username | VARCHAR(64) UNIQUE | |
| password_hash | VARCHAR(255) | bcrypt |
| nickname | VARCHAR(64) | |
| avatar | VARCHAR(512) | URL |
| created_at | DATETIME | |
| updated_at | DATETIME | |

## user_devices
| id | user_id | device_id | device_name | last_sync_seq | last_active_at |

## friend_requests
| id | from_user_id | to_user_id | message | status | created_at |
status: 0=pending 1=accepted 2=rejected

## friendships
| id | user_id | friend_id | created_at | UNIQUE(user_id,friend_id) |

## groups
| id | name | owner_id | avatar | created_at |

## group_members
| id | group_id | user_id | role | joined_at |
role: 0=member 1=admin 2=owner

## conversations
| id | type | created_at | updated_at |
type: 1=direct 2=group

## conversation_members
| id | conversation_id | user_id | last_read_seq |

## messages
| id | conversation_id | from_user_id | seq | msg_type | content | client_msg_id | created_at |
INDEX(conversation_id, seq)

## offline_inbox
| id | user_id | msg_id | created_at |

## files
| id | uploader_id | filename | size | content_type | object_key | status | created_at |
