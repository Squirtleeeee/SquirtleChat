-- Group number, invitations, face-to-face join sessions
ALTER TABLE `groups`
  ADD COLUMN group_no CHAR(4) NULL AFTER name;

CREATE TABLE IF NOT EXISTS group_invitations (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    group_id BIGINT NOT NULL,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    message VARCHAR(255) NOT NULL DEFAULT '',
    invite_type TINYINT NOT NULL DEFAULT 0,
    status TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_to_status (to_user_id, status),
    INDEX idx_group (group_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS group_face_sessions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    group_id BIGINT NOT NULL,
    code CHAR(4) NOT NULL,
    created_by BIGINT NOT NULL,
    expires_at DATETIME(3) NOT NULL,
    INDEX idx_code_exp (code, expires_at)
) ENGINE=InnoDB;
