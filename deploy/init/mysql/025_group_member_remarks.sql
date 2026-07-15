-- Phase 145: per-viewer remarks for group members
CREATE TABLE IF NOT EXISTS group_member_remarks (
  user_id BIGINT NOT NULL,
  group_id BIGINT NOT NULL,
  target_user_id BIGINT NOT NULL,
  remark VARCHAR(64) NOT NULL DEFAULT '',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id, group_id, target_user_id),
  KEY idx_gmr_group (group_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
