-- Create "resources" table
CREATE TABLE "payment"."resources" (
  "id" uuid NOT NULL,
  "merchant_id" uuid NOT NULL,
  "mode" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "resources_mode_check" CHECK (mode = ANY (ARRAY['test'::text, 'live'::text]))
);
