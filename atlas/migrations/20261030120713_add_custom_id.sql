-- Modify "users" table
ALTER TABLE "public"."users" ADD COLUMN "custom_id" character varying(16) NOT NULL DEFAULT substr(md5((random())::text), 1, 15);
