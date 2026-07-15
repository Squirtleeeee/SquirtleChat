-- Phase 144: sync desktop notify / quiet hours prefs
ALTER TABLE user_chat_prefs
  ADD COLUMN notify_json JSON NULL;
