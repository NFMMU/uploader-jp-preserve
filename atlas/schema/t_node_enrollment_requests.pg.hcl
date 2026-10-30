
table "t_node_enrollment_requests" {
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
  column "requested_capacity_bytes" {
    type = bigint
    null = false
    default = 0
  }
  column "contact_email" {
    type = varchar(320)
    null = false
  }
  column "message" {
    type = text
    null = false
    default = ""
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "pending"
  }
  column "terms_accepted_at" {
    type = timestamptz
    null = false
  }
  column "enrollment_token_hash" {
    type = varchar(64)
    null = true
  }
  column "token_expires_at" {
    type = timestamptz
    null = true
  }
  column "approved_by" {
    type = uuid
    null = true
  }
  column "reviewed_by" {
    type = uuid
    null = true
  }
  column "approved_at" {
    type = timestamptz
    null = true
  }
  column "node_id" {
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
  index "node_enrollment_requests_status_created_idx" {
    columns = [column.status, column.created_at]
  }
  index "node_enrollment_requests_token_hash_key" {
    unique = true
    columns = [column.enrollment_token_hash]
  }
  foreign_key "node_enrollment_requests_approved_by_fkey" {
    columns = [column.approved_by]
    ref_columns = [table.m_users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "node_enrollment_requests_reviewed_by_fkey" {
    columns = [column.reviewed_by]
    ref_columns = [table.m_users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "node_enrollment_requests_node_id_fkey" {
    columns = [column.node_id]
    ref_columns = [table.m_nodes.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  check "node_enrollment_requests_type_check" {
    expr = "node_type IN ('A', 'B')"
  }
  check "node_enrollment_requests_capacity_check" {
    expr = "requested_capacity_bytes >= 0"
  }
  check "node_enrollment_requests_status_check" {
    expr = "status IN ('pending', 'approved', 'rejected', 'enrolled')"
  }
  check "node_enrollment_requests_token_state_check" {
    expr = "(status = 'approved' AND enrollment_token_hash IS NOT NULL AND token_expires_at IS NOT NULL) OR (status <> 'approved' AND enrollment_token_hash IS NULL AND token_expires_at IS NULL)"
  }
}