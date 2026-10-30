table "m_nodes" {
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
  column "deleted_at" {
    type = timestamptz
    null = true
    default = null
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
