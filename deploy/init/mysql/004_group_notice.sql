-- Group announcement (owner-editable)
ALTER TABLE `groups`
  ADD COLUMN notice VARCHAR(500) NOT NULL DEFAULT '' AFTER avatar;
