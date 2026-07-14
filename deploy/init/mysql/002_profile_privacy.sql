USE squirtlechat;

ALTER TABLE users ADD COLUMN gender TINYINT NOT NULL DEFAULT 0 AFTER avatar;
ALTER TABLE users ADD COLUMN birthday DATE NULL AFTER gender;
ALTER TABLE users ADD COLUMN privacy_json TEXT NULL AFTER birthday;
UPDATE users SET privacy_json = '{"show_nickname":true,"show_gender":false,"show_birthday":false,"show_avatar":true}' WHERE privacy_json IS NULL OR privacy_json = '';
