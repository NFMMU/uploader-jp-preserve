table "t_policies" {
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
    ref_columns = [table.m_file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "policies_checked_by_fkey" {
    columns = [column.checked_by]
    ref_columns = [table.m_users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "policies_file_object_id_idx" {
    columns = [column.file_object_id]
  }
}