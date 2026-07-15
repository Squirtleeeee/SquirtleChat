USE squirtlechat;

CREATE TABLE IF NOT EXISTS conversation_pins (
    conversation_id VARCHAR(64) NOT NULL,
    msg_id BIGINT NOT NULL,
    pinned_by BIGINT NOT NULL,
    pinned_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (conversation_id, msg_id),
    INDEX idx_conv_pinned_at (conversation_id, pinned_at DESC)
) ENGINE=InnoDB;
