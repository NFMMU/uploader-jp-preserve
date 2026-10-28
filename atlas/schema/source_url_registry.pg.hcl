table "source_url_registry" {
  schema = schema.public
  column "normalized_url" {
    type = text
    null = false
  }
  column "source_url" {
    type = text
    null = false
  }
  column "origin" {
    type = varchar(32)
    null = false
  }
  column "public_request_id" {
    type = uuid
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
    columns = [column.normalized_url]
  }
  foreign_key "source_url_registry_public_request_id_fkey" {
    columns = [column.public_request_id]
    ref_columns = [table.public_requests.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "source_url_registry_request_idx" {
    columns = [column.public_request_id]
  }
  check "source_url_registry_origin_check" {
    expr = "origin IN ('public_request', 'auto_collection', 'catalogued')"
  }
}
