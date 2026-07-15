-- Phase 141: per-group member nickname
ALTER TABLE group_members
  ADD COLUMN nickname VARCHAR(64) NOT NULL DEFAULT '' AFTER muted;
