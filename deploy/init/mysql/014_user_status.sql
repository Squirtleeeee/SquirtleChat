USE squirtlechat;

ALTER TABLE users
  ADD COLUMN status_text VARCHAR(64) NOT NULL DEFAULT '' AFTER avatar,
  ADD COLUMN status_emoji VARCHAR(16) NOT NULL DEFAULT '' AFTER status_text;
