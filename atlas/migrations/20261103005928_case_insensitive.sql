-- Create "case_insensitive" collation
CREATE COLLATION IF NOT EXISTS "case_insensitive" (provider = icu, locale = 'und-u-ks-level1', DETERMINISTIC = FALSE);

-- Create "case_strict_insensitive" collation
CREATE COLLATION IF NOT EXISTS "case_strict_insensitive" (provider = icu, locale = 'und-u-ks-level2', DETERMINISTIC = FALSE);

-- Modify "artifacts" table
ALTER TABLE "public"."artifacts"
ALTER COLUMN "title" TYPE CHARACTER VARYING(500) COLLATE "case_insensitive",
ALTER COLUMN "creator_name" TYPE CHARACTER VARYING(255) COLLATE "case_insensitive",
ALTER COLUMN "description" TYPE text COLLATE "case_insensitive";

-- Modify "download_audit_logs" table
ALTER TABLE "public"."download_audit_logs"
ALTER COLUMN "reason" TYPE text COLLATE "case_insensitive";

-- Modify "file_objects" table
ALTER TABLE "public"."file_objects"
ALTER COLUMN "original_filename" TYPE CHARACTER VARYING(500) COLLATE "case_insensitive";

-- Modify "node_enrollment_requests" table
ALTER TABLE "public"."node_enrollment_requests"
ALTER COLUMN "name" TYPE CHARACTER VARYING(120) COLLATE "case_insensitive",
ALTER COLUMN "contact_email" TYPE CHARACTER VARYING(320) COLLATE "case_insensitive",
ALTER COLUMN "message" TYPE text COLLATE "case_insensitive";

-- Modify "nodes" table
ALTER TABLE "public"."nodes"
ALTER COLUMN "name" TYPE CHARACTER VARYING(120) COLLATE "case_insensitive";

-- Modify "policies" table
ALTER TABLE "public"."policies"
ALTER COLUMN "policy_type" TYPE CHARACTER VARYING(32) COLLATE "case_insensitive",
ALTER COLUMN "evidence_note" TYPE text COLLATE "case_insensitive";

-- Modify "public_requests" table
ALTER TABLE "public"."public_requests"
ALTER COLUMN "submitter_name" TYPE CHARACTER VARYING(255) COLLATE "case_insensitive",
ALTER COLUMN "contact" TYPE CHARACTER VARYING(500) COLLATE "case_insensitive",
ALTER COLUMN "details" TYPE text COLLATE "case_insensitive",
ALTER COLUMN "external_url" TYPE text COLLATE "case_insensitive",
ALTER COLUMN "title" TYPE CHARACTER VARYING(500) COLLATE "case_insensitive",
ALTER COLUMN "creator_name" TYPE CHARACTER VARYING(255) COLLATE "case_insensitive",
ALTER COLUMN "rights_holder_contact" TYPE CHARACTER VARYING(1000) COLLATE "case_insensitive",
ALTER COLUMN "intake_file_path" TYPE text COLLATE "case_insensitive",
ALTER COLUMN "intake_file_name" TYPE CHARACTER VARYING(500) COLLATE "case_insensitive";

-- Modify "request_conversation_links" table
ALTER TABLE "public"."request_conversation_links"
ALTER COLUMN "target_type" TYPE CHARACTER VARYING(32) COLLATE "case_insensitive",
ALTER COLUMN "label" TYPE CHARACTER VARYING(255) COLLATE "case_insensitive";

-- Modify "tasks" table
ALTER TABLE "public"."tasks"
ALTER COLUMN "last_error" TYPE text COLLATE "case_insensitive";

-- Modify "users" table
ALTER TABLE "public"."users"
ALTER COLUMN "email" TYPE CHARACTER VARYING(320) COLLATE "case_strict_insensitive",
ALTER COLUMN "display_name" TYPE CHARACTER VARYING(120) COLLATE "case_insensitive";