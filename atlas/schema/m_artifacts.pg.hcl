table "m_artifacts" {
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
  column "deleted_at" {
    type = timestamptz
    null = true
    default = null
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