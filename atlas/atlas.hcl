env "local" {
  url = getenv("DATABASE_URL")
  dev = "docker://postgres/18/dev"
  schema {
    src = "file://schema"
  }
  migration {
    dir = "file://migrations"
  }
}