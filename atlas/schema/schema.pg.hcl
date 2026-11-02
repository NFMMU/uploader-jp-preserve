schema "public" {}

collation "case_insensitive" {
  schema        = schema.public
  provider      = icu
  locale        = "und-u-ks-level1"
  deterministic = false
}