-- Initial schema for uploader.jp archive catalog and preservation control plane.
CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email varchar(320) NOT NULL,
  password_hash varchar(255) NOT NULL,
  display_name varchar(120) NOT NULL,
  role varchar(32) NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'reviewer', 'viewer')),
  disabled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (email);

CREATE TABLE artifacts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title varchar(500) NOT NULL,
  creator_name varchar(255),
  source_url text NOT NULL,
  description text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX artifacts_title_idx ON artifacts (title);
CREATE INDEX artifacts_creator_name_idx ON artifacts (creator_name);

CREATE TABLE file_objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  artifact_id uuid NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
  original_filename varchar(500) NOT NULL,
  size_bytes bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  sha256 varchar(64),
  media_type varchar(255),
  preservation_status varchar(32) NOT NULL DEFAULT 'uncollected'
    CHECK (preservation_status IN ('uncollected', 'queued', 'stored', 'verified', 'degraded', 'missing', 'error')),
  redistribution_policy varchar(32) NOT NULL DEFAULT 'unknown'
    CHECK (redistribution_policy IN ('allowed', 'restricted', 'in_progress', 'pending', 'unknown')),
  representative uuid REFERENCES users(id) ON DELETE SET NULL,
  public_download_enabled boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX file_objects_artifact_id_idx ON file_objects (artifact_id);
CREATE INDEX file_objects_redistribution_policy_idx ON file_objects (redistribution_policy);

CREATE TABLE policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  file_object_id uuid NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  policy_type varchar(32) NOT NULL,
  evidence_url text,
  evidence_note text,
  checked_at timestamptz,
  checked_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX policies_file_object_id_idx ON policies (file_object_id);

CREATE TABLE nodes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name varchar(120) NOT NULL,
  node_type varchar(8) NOT NULL,
  status varchar(32) NOT NULL DEFAULT 'offline',
  capacity_bytes bigint NOT NULL DEFAULT 0,
  used_bytes bigint NOT NULL DEFAULT 0,
  available_bytes bigint NOT NULL DEFAULT 0,
  requested_capacity_bytes bigint NOT NULL DEFAULT 0,
  last_seen_at timestamptz,
  agent_version varchar(64),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT nodes_type_check CHECK (node_type IN ('A', 'B')),
  CONSTRAINT nodes_capacity_nonnegative_check CHECK (capacity_bytes >= 0 AND used_bytes >= 0 AND available_bytes >= 0 AND requested_capacity_bytes >= 0)
);
CREATE INDEX nodes_status_idx ON nodes (status);

CREATE TABLE placements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  file_object_id uuid NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  node_id uuid NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  storage_key text NOT NULL,
  sha256 varchar(64),
  status varchar(32) NOT NULL DEFAULT 'queued',
  verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX placements_file_node_key ON placements (file_object_id, node_id);
CREATE INDEX placements_file_object_id_idx ON placements (file_object_id);
CREATE INDEX placements_node_id_idx ON placements (node_id);

CREATE TABLE tasks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  task_type varchar(32) NOT NULL,
  file_object_id uuid NOT NULL REFERENCES file_objects(id) ON DELETE CASCADE,
  source_node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  target_node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  status varchar(32) NOT NULL DEFAULT 'queued',
  priority smallint NOT NULL DEFAULT 3 CHECK (priority BETWEEN 0 AND 5),
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_status_priority_created_idx ON tasks (status, priority, created_at);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
  actor_label varchar(320) NOT NULL,
  target_type varchar(64) NOT NULL,
  target_id varchar(128),
  action varchar(100) NOT NULL,
  result varchar(32) NOT NULL,
  reason text,
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_created_at_idx ON audit_logs (created_at);
CREATE INDEX audit_logs_target_idx ON audit_logs (target_type, target_id);

CREATE TABLE public_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_type varchar(32) NOT NULL,
  status varchar(32) NOT NULL DEFAULT 'pending',
  artifact_id uuid REFERENCES artifacts(id) ON DELETE SET NULL,
  file_object_id uuid REFERENCES file_objects(id) ON DELETE SET NULL,
  submitter_name varchar(255),
  contact varchar(500),
  details text NOT NULL,
  external_url text,
  reviewed_by uuid REFERENCES users(id) ON DELETE SET NULL,
  reviewed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT public_requests_type_check CHECK (request_type IN ('takedown', 'deletion', 'external_publication'))
);
CREATE INDEX public_requests_status_created_idx ON public_requests (status, created_at);
