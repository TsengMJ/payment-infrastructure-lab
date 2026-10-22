-- Modify "api_keys" table
ALTER TABLE "merchant"."api_keys" ADD COLUMN "revoked_at" timestamptz NULL;
-- Modify "merchants" table
ALTER TABLE "merchant"."merchants" ADD COLUMN "live_access" boolean NOT NULL DEFAULT false;
-- Create "admin_events" table
CREATE TABLE "merchant"."admin_events" (
  "event_id" uuid NOT NULL,
  "merchant_id" uuid NOT NULL,
  "key_id" text NULL,
  "action" text NOT NULL,
  "mode" text NULL,
  "live_access" boolean NULL,
  "actor" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("event_id"),
  CONSTRAINT "admin_events_merchant_id_fkey" FOREIGN KEY ("merchant_id") REFERENCES "merchant"."merchants" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT,
  CONSTRAINT "admin_events_actor_length" CHECK ((char_length(actor) >= 1) AND (char_length(actor) <= 100)),
  CONSTRAINT "admin_events_shape" CHECK (((action = ANY (ARRAY['api_key.created'::text, 'api_key.revoked'::text])) AND (key_id IS NOT NULL) AND (char_length(key_id) = 22) AND (mode = ANY (ARRAY['test'::text, 'live'::text])) AND (live_access IS NULL)) OR ((action = 'merchant.live_access_set'::text) AND (key_id IS NULL) AND (mode IS NULL) AND (live_access IS NOT NULL)))
);
-- Create index "admin_events_merchant_id_idx" to table: "admin_events"
CREATE INDEX "admin_events_merchant_id_idx" ON "merchant"."admin_events" ("merchant_id");
-- Atlas Community cannot declare these functions in schema/merchant.hcl.
-- They are excluded in atlas.hcl so a later diff does not drop them.
CREATE FUNCTION "merchant"."reject_api_key_update"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.revoked_at IS NOT NULL THEN
    RAISE EXCEPTION 'api key revocation is irreversible';
  END IF;
  IF NEW.key_id IS DISTINCT FROM OLD.key_id
     OR NEW.merchant_id IS DISTINCT FROM OLD.merchant_id
     OR NEW.mode IS DISTINCT FROM OLD.mode
     OR NEW.secret_hash IS DISTINCT FROM OLD.secret_hash
     OR NEW.last4 IS DISTINCT FROM OLD.last4
     OR NEW.name IS DISTINCT FROM OLD.name
     OR NEW.created_at IS DISTINCT FROM OLD.created_at
  THEN
    RAISE EXCEPTION 'api key columns are immutable';
  END IF;
  IF NEW.revoked_at IS NULL OR NEW.revoked_at IS NOT DISTINCT FROM OLD.revoked_at THEN
    RAISE EXCEPTION 'api key revocation must set revoked_at once';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER "api_keys_revoke_only" BEFORE UPDATE ON "merchant"."api_keys" FOR EACH ROW EXECUTE FUNCTION "merchant"."reject_api_key_update"();
CREATE FUNCTION "merchant"."reject_admin_event_change"() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'admin events are append-only';
END;
$$;
CREATE TRIGGER "admin_events_append_only" BEFORE UPDATE OR DELETE ON "merchant"."admin_events" FOR EACH ROW EXECUTE FUNCTION "merchant"."reject_admin_event_change"();
