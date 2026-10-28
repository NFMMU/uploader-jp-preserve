table "users" {
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
  primary_key {
    columns = [column.id]
  }
  index "users_email_key" {
    unique = true
    columns = [column.email]
  }
  check "users_role_check" {
    expr = "role IN ('admin', 'reviewer', 'viewer')"
  }
}