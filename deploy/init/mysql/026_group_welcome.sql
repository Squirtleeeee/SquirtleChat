-- Phase 147: group welcome text
ALTER TABLE `groups`
  ADD COLUMN welcome_text VARCHAR(200) NOT NULL DEFAULT '' AFTER notice;
