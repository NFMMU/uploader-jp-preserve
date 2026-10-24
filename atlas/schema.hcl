schema "public" {}

table "users" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "email" {
    type = varchar(320)
    null = false
  }
  column "password_hash" {
    type = varchar(255)
    null = false
  }
  column "display_name" {
    type = varchar(120)
    null = false
  }
  column "role" {
    type = varchar(32)
    null = false
    default = "viewer"
  }
  column "disabled_at" {
    type = timestamptz
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "users_email_key" {
    unique = true
    columns = [column.email]
  }
  check "users_role_check" {
    expr = "role IN ('admin', 'reviewer', 'viewer')"
  }
}

table "artifacts" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "title" {
    type = varchar(500)
    null = false
  }
  column "creator_name" {
    type = varchar(255)
    null = true
  }
  column "source_url" {
    type = text
    null = false
  }
  column "description" {
    type = text
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "artifacts_title_idx" {
    columns = [column.title]
  }
  index "artifacts_creator_name_idx" {
    columns = [column.creator_name]
  }
}

table "file_objects" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "artifact_id" {
    type = uuid
    null = false
  }
  column "original_filename" {
    type = varchar(500)
    null = false
  }
  column "size_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "sha256" {
    type = varchar(64)
    null = true
  }
  column "media_type" {
    type = varchar(255)
    null = true
  }
  column "preservation_status" {
    type = varchar(32)
    null = false
    default = "uncollected"
  }
  column "redistribution_policy" {
    type = varchar(32)
    null = false
    default = "unknown"
  }
  column "representative" {
    type = uuid
    null = true
  }
  column "public_download_enabled" {
    type = boolean
    null = false
    default = false
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "file_objects_artifact_id_fkey" {
    columns = [column.artifact_id]
    ref_columns = [table.artifacts.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "file_objects_representative_fkey" {
    columns = [column.representative]
    ref_columns = [table.users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "file_objects_artifact_id_idx" {
    columns = [column.artifact_id]
  }
  index "file_objects_redistribution_policy_idx" {
    columns = [column.redistribution_policy]
  }
  check "file_objects_preservation_status_check" {
    expr = "preservation_status IN ('uncollected', 'queued', 'stored', 'verified', 'degraded', 'missing', 'error')"
  }
  check "file_objects_redistribution_policy_check" {
    expr = "redistribution_policy IN ('allowed', 'restricted', 'in_progress', 'pending', 'unknown')"
  }
  check "file_objects_size_bytes_check" {
    expr = "size_bytes >= 0"
  }
}

table "policies" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "file_object_id" {
    type = uuid
    null = false
  }
  column "policy_type" {
    type = varchar(32)
    null = false
  }
  column "evidence_url" {
    type = text
    null = true
  }
  column "evidence_note" {
    type = text
    null = true
  }
  column "checked_at" {
    type = timestamptz
    null = true
  }
  column "checked_by" {
    type = uuid
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "policies_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "policies_checked_by_fkey" {
    columns = [column.checked_by]
    ref_columns = [table.users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "policies_file_object_id_idx" {
    columns = [column.file_object_id]
  }
}

table "nodes" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "name" {
    type = varchar(120)
    null = false
  }
  column "node_type" {
    type = varchar(8)
    null = false
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "offline"
  }
  column "capacity_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "used_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "available_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "requested_capacity_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "last_seen_at" {
    type = timestamptz
    null = true
  }
  column "online_since" {
    type = timestamptz
    null = true
  }
  column "agent_version" {
    type = varchar(64)
    null = true
  }
  column "api_token_hash" {
    type = varchar(64)
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  index "nodes_status_idx" {
    columns = [column.status]
  }
  index "nodes_api_token_hash_key" {
    unique = true
    columns = [column.api_token_hash]
  }
  check "nodes_type_check" {
    expr = "node_type IN ('A', 'B')"
  }
  check "nodes_capacity_nonnegative_check" {
    expr = "capacity_bytes >= 0 AND used_bytes >= 0 AND available_bytes >= 0 AND requested_capacity_bytes >= 0"
  }
}

table "placements" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "file_object_id" {
    type = uuid
    null = false
  }
  column "node_id" {
    type = uuid
    null = false
  }
  column "storage_key" {
    type = text
    null = false
  }
  column "sha256" {
    type = varchar(64)
    null = true
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "queued"
  }
  column "verified_at" {
    type = timestamptz
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "placements_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "placements_node_id_fkey" {
    columns = [column.node_id]
    ref_columns = [table.nodes.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  index "placements_file_object_id_idx" {
    columns = [column.file_object_id]
  }
  index "placements_node_id_idx" {
    columns = [column.node_id]
  }
  index "placements_file_node_key" {
    unique = true
    columns = [column.file_object_id, column.node_id]
  }
}

table "tasks" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "task_type" {
    type = varchar(32)
    null = false
  }
  column "file_object_id" {
    type = uuid
    null = false
  }
  column "source_node_id" {
    type = uuid
    null = true
  }
  column "target_node_id" {
    type = uuid
    null = true
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "queued"
  }
  column "priority" {
    type = smallint
    null = false
    default = 3
  }
  column "attempt_count" {
    type = integer
    null = false
    default = 0
  }
  column "last_error" {
    type = text
    null = true
  }
  column "lease_until" {
    type = timestamptz
    null = true
  }
  column "lease_token" {
    type = uuid
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "tasks_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "tasks_source_node_id_fkey" {
    columns = [column.source_node_id]
    ref_columns = [table.nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "tasks_target_node_id_fkey" {
    columns = [column.target_node_id]
    ref_columns = [table.nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "tasks_status_priority_created_idx" {
    columns = [column.status, column.priority, column.created_at]
  }
  index "tasks_lease_token_key" {
    unique = true
    columns = [column.lease_token]
  }
  index "tasks_source_node_status_idx" {
    columns = [column.source_node_id, column.status]
  }
  check "tasks_priority_check" {
    expr = "priority BETWEEN 0 AND 5"
  }
  check "tasks_attempt_count_check" {
    expr = "attempt_count >= 0"
  }
}

table "audit_logs" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "actor_id" {
    type = uuid
    null = true
  }
  column "actor_label" {
    type = varchar(320)
    null = false
  }
  column "target_type" {
    type = varchar(64)
    null = false
  }
  column "target_id" {
    type = varchar(128)
    null = true
  }
  column "action" {
    type = varchar(100)
    null = false
  }
  column "result" {
    type = varchar(32)
    null = false
  }
  column "reason" {
    type = text
    null = true
  }
  column "metadata" {
    type = jsonb
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "audit_logs_actor_id_fkey" {
    columns = [column.actor_id]
    ref_columns = [table.users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "audit_logs_created_at_idx" {
    columns = [column.created_at]
  }
  index "audit_logs_target_idx" {
    columns = [column.target_type, column.target_id]
  }
}

table "public_requests" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "request_type" {
    type = varchar(32)
    null = false
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "pending"
  }
  column "artifact_id" {
    type = uuid
    null = true
  }
  column "file_object_id" {
    type = uuid
    null = true
  }
  column "submitter_name" {
    type = varchar(255)
    null = true
  }
  column "contact" {
    type = varchar(500)
    null = true
  }
  column "details" {
    type = text
    null = false
  }
  column "external_url" {
    type = text
    null = true
  }
  column "reviewed_by" {
    type = uuid
    null = true
  }
  column "reviewed_at" {
    type = timestamptz
    null = true
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  column "updated_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "public_requests_artifact_id_fkey" {
    columns = [column.artifact_id]
    ref_columns = [table.artifacts.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "public_requests_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.file_objects.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "public_requests_reviewed_by_fkey" {
    columns = [column.reviewed_by]
    ref_columns = [table.users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "public_requests_status_created_idx" {
    columns = [column.status, column.created_at]
  }
  check "public_requests_type_check" {
    expr = "request_type IN ('takedown', 'deletion', 'external_publication')"
  }
}
