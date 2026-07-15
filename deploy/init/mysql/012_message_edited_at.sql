USE squirtlechat;

ALTER TABLE messages
  ADD COLUMN edited_at DATETIME(3) NULL DEFAULT NULL AFTER created_at;
