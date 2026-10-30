table "t_public_requests" {
  schema = schema.public
  column "id" {
    type = uuid
    null = false
    default = sql("gen_random_uuid()")
  }
  column "request_type" {
    type = varchar(32)
    null = false
  }
  column "status" {
    type = varchar(32)
    null = false
    default = "pending"
  }
  column "artifact_id" {
    type = uuid
    null = true
  }
  column "file_object_id" {
    type = uuid
    null = true
  }
  column "title" {
    type = varchar(500)
    null = true
  }
  column "creator_name" {
    type = varchar(255)
    null = true
  }
  column "source_url" {
    type = text
    null = true
  }
  column "source_password_encrypted" {
    type = text
    null = true
  }
  column "rights_holder_contact" {
    type = varchar(1000)
    null = true
  }
  column "intake_file_path" {
    type = text
    null = false
    default = ""
  }
  column "intake_file_name" {
    type = varchar(500)
    null = true
  }
  column "intake_file_size" {
    type = bigint
    null = true
  }
  column "intake_file_sha256" {
    type = varchar(64)
    null = true
  }
  column "intake_file_uploaded_at" {
    type = timestamptz
    null = true
  }
  column "submitter_name" {
    type = varchar(255)
    null = true
  }
  column "contact" {
    type = varchar(500)
    null = true
  }
  column "details" {
    type = text
    null = false
  }
  column "external_url" {
    type = text
    null = true
  }
  column "reviewed_by" {
    type = uuid
    null = true
  }
  column "reviewed_at" {
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
  foreign_key "public_requests_artifact_id_fkey" {
    columns = [column.artifact_id]
    ref_columns = [table.m_artifacts.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "public_requests_file_object_id_fkey" {
    columns = [column.file_object_id]
    ref_columns = [table.m_file_objects.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  foreign_key "public_requests_reviewed_by_fkey" {
    columns = [column.reviewed_by]
    ref_columns = [table.m_users.column.id]
    on_update = NO_ACTION
    on_delete = SET_NULL
  }
  index "public_requests_status_created_idx" {
    columns = [column.status, column.created_at]
  }
  check "public_requests_type_check" {
    expr = "request_type IN ('takedown', 'deletion', 'external_publication', 'preservation', 'redistribution_assistance', 'publisher_assistance')"
  }
}
