schema "merchant" {}

table "merchants" {
  schema = schema.merchant
  column "id" {
    null = false
    type = uuid
  }
  column "name" {
    null = false
    type = text
  }
  column "status" {
    null = false
    type = text
  }
  column "contact_email" {
    null = true
    type = sql("varchar(254)")
  }
  column "metadata" {
    null    = false
    type    = jsonb
    default = sql("'{}'::jsonb")
  }
  column "created_at" {
    null = false
    type = timestamptz
  }
  column "updated_at" {
    null = false
    type = timestamptz
  }
  primary_key {
    columns = [column.id]
  }
  check "merchants_name_length" {
    expr = "(char_length(name) BETWEEN 1 AND 200)"
  }
  check "merchants_status_check" {
    expr = "(status IN ('active', 'suspended', 'closed'))"
  }
}

table "api_keys" {
  schema = schema.merchant
  column "key_id" {
    null = false
    type = text
  }
  column "merchant_id" {
    null = false
    type = uuid
  }
  column "mode" {
    null = false
    type = text
  }
  column "secret_hash" {
    null = false
    type = bytea
  }
  column "last4" {
    null = false
    type = text
  }
  column "name" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = timestamptz
  }
  primary_key {
    columns = [column.key_id]
  }
  foreign_key "api_keys_merchant_id_fkey" {
    columns     = [column.merchant_id]
    ref_columns = [table.merchants.column.id]
    on_update   = NO_ACTION
    on_delete   = RESTRICT
  }
  index "api_keys_merchant_id_idx" {
    columns = [column.merchant_id]
  }
  check "api_keys_mode_check" {
    expr = "(mode IN ('test', 'live'))"
  }
  check "api_keys_secret_hash_len" {
    expr = "(octet_length(secret_hash) = 32)"
  }
  check "api_keys_last4_len" {
    expr = "(char_length(last4) = 4)"
  }
  check "api_keys_name_length" {
    expr = "(char_length(name) BETWEEN 1 AND 100)"
  }
}
