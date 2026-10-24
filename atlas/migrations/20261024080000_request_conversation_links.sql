-- Stores one or more LibreDesk/support conversation URLs per incoming request.
-- The polymorphic target is validated by the API against the supported request tables.
CREATE TABLE request_conversation_links (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  target_type varchar(32) NOT NULL CHECK (target_type IN ('public_request', 'node_enrollment_request')),
  target_id uuid NOT NULL,
  url text NOT NULL,
  label varchar(255) NOT NULL DEFAULT '関連する会話',
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT request_conversation_links_target_url_key UNIQUE (target_type, target_id, url)
);
CREATE INDEX request_conversation_links_target_idx ON request_conversation_links (target_type, target_id, created_at);
