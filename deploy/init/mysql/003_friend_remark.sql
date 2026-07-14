USE squirtlechat;

ALTER TABLE friendships ADD COLUMN remark VARCHAR(64) NOT NULL DEFAULT '' AFTER friend_id;
