USE squirtlechat;

CREATE TABLE IF NOT EXISTS user_chat_prefs (
    user_id BIGINT PRIMARY KEY,
    muted_json JSON NOT NULL,
    pinned_friends_json JSON NOT NULL,
    pinned_groups_json JSON NOT NULL,
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB;
