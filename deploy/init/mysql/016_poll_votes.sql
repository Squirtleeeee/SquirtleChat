USE squirtlechat;

CREATE TABLE IF NOT EXISTS poll_votes (
    msg_id BIGINT NOT NULL,
    conversation_id VARCHAR(64) NOT NULL,
    option_id VARCHAR(32) NOT NULL,
    user_id BIGINT NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (msg_id, user_id),
    INDEX idx_poll_msg (msg_id),
    INDEX idx_poll_conv (conversation_id, msg_id)
) ENGINE=InnoDB;
