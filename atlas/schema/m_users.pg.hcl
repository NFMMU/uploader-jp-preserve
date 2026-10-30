table "m_users" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "email" {
    type = varchar(320)
    null = false
  }
  column "password_hash" {
    type = varchar(255)
    null = false
  }
  column "display_name" {
    type = varchar(120)
    null = false
  }
  column "custom_id" {
    type = varchar(16)
    null = false
    default = sql("substr(md5(random()::text), 1, 15)")
  }
  column "role" {
    type = varchar(32)
    null = false
    default = "viewer"
  }
  column "disabled_at" {
    type = timestamptz
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
  index "users_email_key" {
    unique = true
    columns = [column.email]
  }
  index "users_custom_id_key" {
    unique = true
    columns = [column.custom_id]
  }
  check "users_role_check" {
    expr = "role IN ('admin', 'reviewer', 'viewer', 'author')"
  }
}