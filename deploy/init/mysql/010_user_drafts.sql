USE squirtlechat;

CREATE TABLE IF NOT EXISTS user_drafts (
    user_id BIGINT NOT NULL,
    conversation_id VARCHAR(64) NOT NULL,
    content TEXT NOT NULL,
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, conversation_id),
    INDEX idx_user_draft_updated (user_id, updated_at DESC)
) ENGINE=InnoDB;
