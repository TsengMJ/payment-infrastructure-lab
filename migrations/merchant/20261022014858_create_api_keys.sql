-- Create "api_keys" table
CREATE TABLE "merchant"."api_keys" (
  "key_id" text NOT NULL,
  "merchant_id" uuid NOT NULL,
  "mode" text NOT NULL,
  "secret_hash" bytea NOT NULL,
  "last4" text NOT NULL,
  "name" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("key_id"),
  CONSTRAINT "api_keys_merchant_id_fkey" FOREIGN KEY ("merchant_id") REFERENCES "merchant"."merchants" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "api_keys_last4_len" CHECK (char_length(last4) = 4),
  CONSTRAINT "api_keys_mode_check" CHECK (mode = ANY (ARRAY['test'::text, 'live'::text])),
  CONSTRAINT "api_keys_name_length" CHECK ((char_length(name) >= 1) AND (char_length(name) <= 100)),
  CONSTRAINT "api_keys_secret_hash_len" CHECK (octet_length(secret_hash) = 32)
);
-- Create index "api_keys_merchant_id_idx" to table: "api_keys"
CREATE INDEX "api_keys_merchant_id_idx" ON "merchant"."api_keys" ("merchant_id");
