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
}