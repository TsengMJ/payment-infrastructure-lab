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
