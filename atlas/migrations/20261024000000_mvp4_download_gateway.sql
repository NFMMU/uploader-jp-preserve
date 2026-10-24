-- MVP-4: isolated public-download preparation and delivery audit.
CREATE TABLE download_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  file_object_id uuid NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  selected_node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  status varchar(16) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','ready','failed','expired')),
  access_token_hash varchar(64) NOT NULL,
  lease_token uuid,
  lease_until timestamptz,
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  size_bytes bigint,
  sha256 varchar(64),
  last_error text,
  expires_at timestamptz NOT NULL DEFAULT (now() + interval '30 minutes'),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX download_jobs_status_created_idx ON download_jobs (status, created_at);
CREATE INDEX download_jobs_file_idx ON download_jobs (file_object_id);
CREATE INDEX download_jobs_expiry_idx ON download_jobs (expires_at);

CREATE TABLE download_audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  download_job_id uuid REFERENCES download_jobs(id) ON DELETE SET NULL,
  file_object_id uuid REFERENCES file_objects(id) ON DELETE SET NULL,
  selected_node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  action varchar(32) NOT NULL,
  result varchar(16) NOT NULL,
  reason text,
  bytes_sent bigint NOT NULL DEFAULT 0,
  duration_ms bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX download_audit_logs_created_idx ON download_audit_logs (created_at);
CREATE INDEX download_audit_logs_job_idx ON download_audit_logs (download_job_id);
