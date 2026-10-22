schema "payment" {}

table "resources" {
  schema = schema.payment
  column "id" {
    null = false
    type = uuid
  }
  column "merchant_id" {
    null = false
    type = uuid
  }
  column "mode" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = timestamptz
  }
  primary_key {
    columns = [column.id]
  }
  check "resources_mode_check" {
    expr = "(mode IN ('test', 'live'))"
  }
}
