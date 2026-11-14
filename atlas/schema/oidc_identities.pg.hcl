table "oidc_identities" {
  schema = schema.public
  column "id" {
    type = uuid
    default = sql("uuidv7()")
    null = false
  }
  column "issuer" {
    type = varchar(255)
    null = false
  }
  column "subject" {
    type = varchar(255)
    null = false
  }
  column "email" {
    type = varchar(255)
    null = false
  }
  column "preferred_username" {
    type = varchar(255)
    null = true
  }
  column "picture" {
    type = varchar(255)
    null = true
  }
  column "provider_id" {
    type = uuid
    null = false
  }
  column "user_id" {
    type = uuid
    null = false
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
  }
  primary_key {
    columns = [ column.id ]
  }
  foreign_key "fk_provider_id" {
    columns = [ column.provider_id ]
    ref_columns = [ table.oidc_providers.column.id ]
  }
  foreign_key "fk_user_id" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
  }
  index "uq_provider_user" {
    unique = true
    where = "deleted_at IS NULL"
    columns = [ column.provider_id, column.user_id ]
  }
  index "uq_provider_subject" {
    unique = true
    where = "deleted_at IS NULL"
    columns = [ column.provider_id, column.subject ]
  }
}