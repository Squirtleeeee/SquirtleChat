USE squirtlechat;

CREATE TABLE IF NOT EXISTS conversation_bookmarks (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id VARCHAR(64) NOT NULL,
    title VARCHAR(128) NOT NULL,
    url VARCHAR(1024) NOT NULL,
    created_by BIGINT NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_conv_bookmark (conversation_id, created_at DESC)
) ENGINE=InnoDB;
