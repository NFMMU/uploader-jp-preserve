-- Modify "users" table
ALTER TABLE "public"."users" ADD COLUMN "deleted_at" timestamptz NULL;
