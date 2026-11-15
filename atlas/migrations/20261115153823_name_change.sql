-- Create "author_artifact" table
CREATE TABLE "public"."author_artifact" (
  "user_id" uuid NOT NULL,
  "artifact_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("artifact_id", "user_id")
);
-- Drop "author_artifacts" table
DROP TABLE "public"."author_artifacts";
