USE squirtlechat;

ALTER TABLE `groups`
  ADD COLUMN admin_only TINYINT NOT NULL DEFAULT 0 COMMENT '1=仅管理员/群主可发言';

ALTER TABLE group_members
  ADD COLUMN muted TINYINT NOT NULL DEFAULT 0 COMMENT '1=被禁言';
