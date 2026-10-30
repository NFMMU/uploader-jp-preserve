-- Create "j_author_artifacts" table
CREATE TABLE "public"."j_author_artifacts" (
  "author_id" uuid NOT NULL,
  "artifact_id" uuid NOT NULL,
  PRIMARY KEY ("author_id", "artifact_id")
);
