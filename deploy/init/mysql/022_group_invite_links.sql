-- Phase 142: revocable group invite links
CREATE TABLE IF NOT EXISTS group_invite_links (
  id BIGINT PRIMARY KEY,
  group_id BIGINT NOT NULL,
  code VARCHAR(16) NOT NULL,
  created_by BIGINT NOT NULL,
  max_uses INT NOT NULL DEFAULT 0,
  use_count INT NOT NULL DEFAULT 0,
  expires_at DATETIME(3) NULL,
  revoked TINYINT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_invite_code (code),
  KEY idx_invite_group (group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
