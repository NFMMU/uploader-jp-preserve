schema "public" {}

collation "case_insensitive" {
  schema        = schema.public
  provider      = icu
  locale        = "und-u-ks-level1"
  deterministic = false
}

collation "case_strict_insensitive" {
  schema        = schema.public
  provider      = icu
  locale        = "und-u-ks-level2"
  deterministic = false
}