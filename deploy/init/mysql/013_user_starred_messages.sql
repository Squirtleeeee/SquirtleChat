USE squirtlechat;

CREATE TABLE IF NOT EXISTS user_starred_messages (
    user_id BIGINT NOT NULL,
    conversation_id VARCHAR(64) NOT NULL,
    msg_id BIGINT NOT NULL,
    starred_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, msg_id),
    INDEX idx_user_starred (user_id, starred_at DESC)
) ENGINE=InnoDB;
