-- Phase 143: group slow mode (seconds between messages for non-admins)
ALTER TABLE `groups`
  ADD COLUMN slow_mode_secs INT NOT NULL DEFAULT 0 AFTER admin_only;
