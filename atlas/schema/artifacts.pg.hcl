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
    collate = collation.case_insensitive
  }
  column "creator_name" {
    type = varchar(255)
    null = true
    collate = collation.case_insensitive
  }
  column "source_url" {
    type = text
    null = false
    collate = collation.case_insensitive
  }
  column "description" {
    type = text
    null = true
    collate = collation.case_insensitive
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