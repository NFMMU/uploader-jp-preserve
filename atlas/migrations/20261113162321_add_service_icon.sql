-- Rename a column from "name" to "display_name"
ALTER TABLE "public"."oidc_providers" RENAME COLUMN "name" TO "display_name";
-- Modify "oidc_providers" table
ALTER TABLE "public"."oidc_providers" ADD COLUMN "service_icon" character varying(255) NULL;
