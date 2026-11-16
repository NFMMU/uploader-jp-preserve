-- Create "author_artifact" table
CREATE TABLE "public"."author_artifact" (
  "user_id" uuid NOT NULL,
  "artifact_id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("artifact_id", "user_id"),
  CONSTRAINT "fk_artifact_id" FOREIGN KEY ("artifact_id") REFERENCES "public"."artifacts" ("id") ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT "fk_user_id" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE CASCADE ON DELETE CASCADE
);
-- Drop "author_artifacts" table
DROP TABLE "public"."author_artifacts";
