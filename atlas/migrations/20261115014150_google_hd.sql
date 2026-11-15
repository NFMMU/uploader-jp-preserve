-- Modify "oidc_providers" table
ALTER TABLE "public"."oidc_providers" ADD COLUMN "google_hd" character varying(255) NULL;
