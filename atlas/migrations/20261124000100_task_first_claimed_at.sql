-- Remember whether a task has ever been claimed. A retried delete task resets
-- attempt_count, so attempt_count alone cannot tell whether the node may have
-- already removed the file.
ALTER TABLE tasks ADD COLUMN first_claimed_at timestamptz;
