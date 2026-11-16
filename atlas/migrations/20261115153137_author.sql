-- Modify "users" table
ALTER TABLE "public"."users" DROP CONSTRAINT "users_role_check", ADD CONSTRAINT "users_role_check" CHECK ((role)::text = ANY ((ARRAY['admin'::character varying, 'reviewer'::character varying, 'viewer'::character varying, 'author'::character varying])::text[]));
-- Create "author_artifacts" table
CREATE TABLE "public"."author_artifacts" (
  "user_id" uuid NOT NULL,
  "artifact_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("artifact_id", "user_id")
);
