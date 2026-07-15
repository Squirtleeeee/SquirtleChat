USE squirtlechat;

CREATE TABLE IF NOT EXISTS message_hashtags (
    conversation_id VARCHAR(64) NOT NULL,
    msg_id BIGINT NOT NULL,
    tag VARCHAR(64) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (msg_id, tag),
    INDEX idx_conv_tag (conversation_id, tag),
    INDEX idx_tag (tag)
) ENGINE=InnoDB;
