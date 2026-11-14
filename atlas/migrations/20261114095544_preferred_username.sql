-- Modify "oidc_identities" table
ALTER TABLE "public"."oidc_identities" ADD COLUMN "preferred_username" character varying(255) NULL, ADD COLUMN "picture" character varying(255) NULL;
