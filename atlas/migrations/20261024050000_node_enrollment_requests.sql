-- Public node participation uses an explicit review step and a per-request, one-time token.
CREATE TABLE node_enrollment_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name varchar(120) NOT NULL,
  node_type varchar(8) NOT NULL CHECK (node_type IN ('A', 'B')),
  requested_capacity_bytes bigint NOT NULL DEFAULT 0 CHECK (requested_capacity_bytes >= 0),
  contact_email varchar(320) NOT NULL,
  message text NOT NULL DEFAULT '',
  status varchar(32) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'approved', 'rejected', 'enrolled')),
  terms_accepted_at timestamptz NOT NULL,
  enrollment_token_hash varchar(64),
  token_expires_at timestamptz,
  approved_by uuid REFERENCES users(id) ON DELETE SET NULL,
  reviewed_by uuid REFERENCES users(id) ON DELETE SET NULL,
  approved_at timestamptz,
  node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((status = 'approved' AND enrollment_token_hash IS NOT NULL AND token_expires_at IS NOT NULL)
      OR (status <> 'approved' AND enrollment_token_hash IS NULL AND token_expires_at IS NULL))
);
CREATE INDEX node_enrollment_requests_status_created_idx ON node_enrollment_requests (status, created_at);
CREATE UNIQUE INDEX node_enrollment_requests_token_hash_key ON node_enrollment_requests (enrollment_token_hash);
