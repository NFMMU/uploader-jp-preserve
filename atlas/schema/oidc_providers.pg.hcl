table "oidc_providers" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("uuidv7()")
  }
  column "display_name" {
    type = varchar(31)
    null = false
  }
  column "service_icon" {
    type = varchar(255)
    null = true
  }
  column "discovery_endpoint" {
    type = varchar(255)
    null = false
  }
  column "client_id" {
    type = varchar(255)
    null = false
  }
  column "client_secret" {
    type = varchar(255)
    null = false
  }
  column "is_enabled" {
    type = bool
    default = true
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
    columns = [column.id]
  }
  unique "uq_client_id_discovery_endpoint" {
    columns = [ column.client_id, column.discovery_endpoint ]
  }
  index "idx_is_enabled" {
    columns = [ column.is_enabled ]
  }
}