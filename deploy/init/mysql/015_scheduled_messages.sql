USE squirtlechat;

CREATE TABLE IF NOT EXISTS scheduled_messages (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    conversation_id VARCHAR(64) NOT NULL,
    conversation_type TINYINT NOT NULL,
    to_user_id BIGINT NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    msg_type TINYINT NOT NULL DEFAULT 1,
    send_at DATETIME(3) NOT NULL,
    status TINYINT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_due (status, send_at),
    INDEX idx_user (user_id, status)
) ENGINE=InnoDB;
