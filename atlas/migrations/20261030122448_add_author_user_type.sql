-- Rename a constraint from "artifacts_pkey" to "m_artifacts_pkey"
ALTER TABLE "public"."m_artifacts" RENAME CONSTRAINT "artifacts_pkey" TO "m_artifacts_pkey";
-- Modify "m_artifacts" table
ALTER TABLE "public"."m_artifacts" ADD COLUMN "deleted_at" timestamptz NULL;
-- Rename a constraint from "file_objects_pkey" to "m_file_objects_pkey"
ALTER TABLE "public"."m_file_objects" RENAME CONSTRAINT "file_objects_pkey" TO "m_file_objects_pkey";
-- Rename a constraint from "nodes_pkey" to "m_nodes_pkey"
ALTER TABLE "public"."m_nodes" RENAME CONSTRAINT "nodes_pkey" TO "m_nodes_pkey";
-- Modify "m_nodes" table
ALTER TABLE "public"."m_nodes" ADD COLUMN "deleted_at" timestamptz NULL;
-- Rename a constraint from "users_pkey" to "m_users_pkey"
ALTER TABLE "public"."m_users" RENAME CONSTRAINT "users_pkey" TO "m_users_pkey";
-- Modify "m_users" table
ALTER TABLE "public"."m_users" DROP CONSTRAINT "users_role_check", ADD CONSTRAINT "users_role_check" CHECK ((role)::text = ANY ((ARRAY['admin'::character varying, 'reviewer'::character varying, 'viewer'::character varying, 'author'::character varying])::text[]));
-- Rename a constraint from "audit_logs_pkey" to "t_audit_logs_pkey"
ALTER TABLE "public"."t_audit_logs" RENAME CONSTRAINT "audit_logs_pkey" TO "t_audit_logs_pkey";
-- Rename a constraint from "download_audit_logs_pkey" to "t_download_audit_logs_pkey"
ALTER TABLE "public"."t_download_audit_logs" RENAME CONSTRAINT "download_audit_logs_pkey" TO "t_download_audit_logs_pkey";
-- Rename a constraint from "download_jobs_pkey" to "t_download_jobs_pkey"
ALTER TABLE "public"."t_download_jobs" RENAME CONSTRAINT "download_jobs_pkey" TO "t_download_jobs_pkey";
-- Rename a constraint from "node_enrollment_requests_pkey" to "t_node_enrollment_requests_pkey"
ALTER TABLE "public"."t_node_enrollment_requests" RENAME CONSTRAINT "node_enrollment_requests_pkey" TO "t_node_enrollment_requests_pkey";
-- Rename a constraint from "placements_pkey" to "t_placements_pkey"
ALTER TABLE "public"."t_placements" RENAME CONSTRAINT "placements_pkey" TO "t_placements_pkey";
-- Rename a constraint from "policies_pkey" to "t_policies_pkey"
ALTER TABLE "public"."t_policies" RENAME CONSTRAINT "policies_pkey" TO "t_policies_pkey";
-- Rename a constraint from "public_requests_pkey" to "t_public_requests_pkey"
ALTER TABLE "public"."t_public_requests" RENAME CONSTRAINT "public_requests_pkey" TO "t_public_requests_pkey";
-- Rename a constraint from "request_conversation_links_pkey" to "t_request_conversation_links_pkey"
ALTER TABLE "public"."t_request_conversation_links" RENAME CONSTRAINT "request_conversation_links_pkey" TO "t_request_conversation_links_pkey";
-- Rename a constraint from "source_url_registry_pkey" to "t_source_url_registry_pkey"
ALTER TABLE "public"."t_source_url_registry" RENAME CONSTRAINT "source_url_registry_pkey" TO "t_source_url_registry_pkey";
-- Rename a constraint from "tasks_pkey" to "t_tasks_pkey"
ALTER TABLE "public"."t_tasks" RENAME CONSTRAINT "tasks_pkey" TO "t_tasks_pkey";
