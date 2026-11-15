-- Modify "oidc_identities" table
ALTER TABLE "public"."oidc_identities" ADD COLUMN "preferred_username" text NULL, ADD COLUMN "picture" text NULL;
