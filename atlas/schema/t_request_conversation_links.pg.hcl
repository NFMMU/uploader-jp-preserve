table "t_request_conversation_links" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "target_type" {
    type = varchar(32)
    null = false
  }
  column "target_id" {
    type = uuid
    null = false
  }
  column "url" {
    type = text
    null = false
  }
  column "label" {
    type = varchar(255)
    null = false
    default = "関連する会話"
  }
  column "created_by" {
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
  foreign_key "request_conversation_links_created_by_fkey" {
    columns = [column.created_by]
    ref_columns = [table.m_users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "request_conversation_links_target_url_key" {
    unique = true
    columns = [column.target_type, column.target_id, column.url]
  }
  index "request_conversation_links_target_idx" {
    columns = [column.target_type, column.target_id, column.created_at]
  }
  check "request_conversation_links_target_type_check" {
    expr = "target_type IN ('public_request', 'node_enrollment_request')"
  }
}
