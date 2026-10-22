variable "database_url" {
  type    = string
  default = getenv("DATABASE_URL")
}

# schemas limits each env to its own module. A database-scoped diff would
# otherwise DROP SCHEMA public (created by Postgres) and any other module.
# skip drop_schema keeps that out of generated files.

env "merchant" {
  url     = var.database_url
  dev     = "docker://postgres/18/dev"
  src     = "file://schema/merchant.hcl"
  schemas = ["merchant"]
  # Functions and triggers are written in the api_key_rotation migration.
  # Community Atlas cannot load them from HCL. Excluding them keeps a later
  # diff from planning a drop.
  exclude = [
    "merchant.reject_api_key_update",
    "merchant.reject_admin_event_change",
    "merchant.api_keys.api_keys_revoke_only",
    "merchant.admin_events.admin_events_append_only",
  ]
  migration {
    dir              = "file://migrations/merchant"
    revisions_schema = "atlas_merchant"
  }
  diff {
    skip {
      drop_schema = true
    }
  }
}

env "payment" {
  url     = var.database_url
  dev     = "docker://postgres/18/dev"
  src     = "file://schema/payment.hcl"
  schemas = ["payment"]
  migration {
    dir              = "file://migrations/payment"
    revisions_schema = "atlas_payment"
  }
  diff {
    skip {
      drop_schema = true
    }
  }
}
