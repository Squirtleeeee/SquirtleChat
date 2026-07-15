USE squirtlechat;

ALTER TABLE user_chat_prefs
  ADD COLUMN folders_json JSON NULL;

UPDATE user_chat_prefs
SET folders_json = JSON_ARRAY()
WHERE folders_json IS NULL;
