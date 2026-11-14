-- Create "oidc_providers" table
CREATE TABLE
  "public"."oidc_providers" (
    "id" uuid NOT NULL DEFAULT uuidv7 (),
    "name" CHARACTER VARYING(31) NOT NULL,
    "discovery_endpoint" CHARACTER VARYING(255) NOT NULL,
    "client_id" CHARACTER VARYING(255) NOT NULL,
    "client_secret" CHARACTER VARYING(255) NOT NULL,
    "is_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "created_at" timestamptz NOT NULL DEFAULT now (),
    "updated_at" timestamptz NOT NULL DEFAULT now (),
    "deleted_at" timestamptz NULL,
    PRIMARY KEY ("id"),
    CONSTRAINT "uq_client_id_discovery_endpoint" UNIQUE ("client_id", "discovery_endpoint")
  );

-- Create index "idx_is_enabled" to table: "oidc_providers"
CREATE INDEX "idx_is_enabled" ON "public"."oidc_providers" ("is_enabled");

-- Create "oidc_identities" table
CREATE TABLE
  "public"."oidc_identities" (
    "id" uuid NOT NULL DEFAULT uuidv7 (),
    "issuer" CHARACTER VARYING(255) NOT NULL,
    "subject" CHARACTER VARYING(255) NOT NULL,
    "email" VARYING (255) NOT NULL,
    "provider_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT now (),
    "updated_at" timestamptz NOT NULL DEFAULT now (),
    "deleted_at" timestamptz NULL,
    PRIMARY KEY ("id"),
    CONSTRAINT "uq_provider_subject" UNIQUE ("provider_id", "subject"),
    CONSTRAINT "uq_provider_user" UNIQUE ("provider_id", "user_id"),
    CONSTRAINT "fk_provider_id" FOREIGN KEY ("provider_id") REFERENCES "public"."oidc_providers" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
    CONSTRAINT "fk_user_id" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
  );