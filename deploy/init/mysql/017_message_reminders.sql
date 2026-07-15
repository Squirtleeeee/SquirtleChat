USE squirtlechat;

CREATE TABLE IF NOT EXISTS message_reminders (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    conversation_id VARCHAR(64) NOT NULL,
    msg_id BIGINT NOT NULL,
    preview VARCHAR(256) NOT NULL DEFAULT '',
    remind_at DATETIME(3) NOT NULL,
    status TINYINT NOT NULL DEFAULT 0 COMMENT '0=pending 1=fired 2=cancelled',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_due (status, remind_at),
    INDEX idx_user (user_id, status)
) ENGINE=InnoDB;
