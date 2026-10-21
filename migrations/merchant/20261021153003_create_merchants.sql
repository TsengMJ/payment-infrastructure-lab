-- Create "merchants" table
CREATE TABLE "merchant"."merchants" (
  "id" uuid NOT NULL,
  "name" text NOT NULL,
  "status" text NOT NULL,
  "contact_email" character varying(254) NULL,
  "metadata" jsonb NOT NULL DEFAULT '{}',
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "merchants_name_length" CHECK ((char_length(name) >= 1) AND (char_length(name) <= 200)),
  CONSTRAINT "merchants_status_check" CHECK (status = ANY (ARRAY['active'::text, 'suspended'::text, 'closed'::text]))
);
