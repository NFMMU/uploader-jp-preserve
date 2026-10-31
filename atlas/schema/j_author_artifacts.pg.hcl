table "j_author_artifacts" {
  schema = schema.public
  column "author_id" {
    type = uuid
    null = false
  }
  column "artifact_id" {
    type = uuid
    null = false
  }
  primary_key {
    columns = [column.author_id, column.artifact_id]
  }
}