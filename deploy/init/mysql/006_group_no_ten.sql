-- Widen group_no to ~10 digits; reset legacy 4-digit values for re-allocation
ALTER TABLE `groups`
  MODIFY COLUMN group_no VARCHAR(16) NULL;

UPDATE `groups` SET group_no = NULL WHERE group_no IS NOT NULL AND CHAR_LENGTH(group_no) < 8;
