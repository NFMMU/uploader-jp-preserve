-- MVP-3: track continuous online time so surplus cleanup waits after node recovery.
ALTER TABLE nodes ADD COLUMN online_since timestamptz;
UPDATE nodes SET online_since = last_seen_at WHERE status = 'online';
