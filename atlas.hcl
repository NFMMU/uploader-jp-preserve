env "local" {
  src = "file://atlas/schema.hcl"
  url = getenv("DATABASE_URL")
  dev = "docker://postgres/16/dev?search_path=public"

  migration {
    dir = "file://atlas/migrations"
    format = atlas
  }
}
