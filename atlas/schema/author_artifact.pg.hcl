table "author_artifact" {
  schema = schema.public
  column "user_id" {
    type = uuid
    null = false
  }
  column "artifact_id" {
    type = uuid
    null = false
  }
  column "created_at" {
    type = timestamptz
    null = false
    default = sql("now()")
  }
  primary_key {
    columns = [ column.artifact_id, column.user_id ]
  }
  foreign_key "fk_artifact_id" {
    columns = [ column.artifact_id ]
    ref_columns = [ table.artifacts.column.id ]
    on_delete = CASCADE
    on_update = CASCADE
  }
  foreign_key "fk_user_id" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete = CASCADE
    on_update = CASCADE
  }
}