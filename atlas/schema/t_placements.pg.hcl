
table "t_placements" {
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
    ref_columns = [table.m_file_objects.column.id]
    on_update = NO_ACTION
    on_delete = CASCADE
  }
  foreign_key "placements_node_id_fkey" {
    columns = [column.node_id]
    ref_columns = [table.m_nodes.column.id]
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