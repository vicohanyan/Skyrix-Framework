# Atlas generates SQL migrations from the GORM models:
#   cobra db migrate:diff <name>      (atlas migrate diff <name> --env local)
# Hand-written migrations are created with `cobra make migration <name>`.
# Migrations are applied by `cobra db migrate`, never by `atlas migrate apply`.

data "external_schema" "gorm" {
  program = ["go", "run", "./tools/schemaloader"]
}

env "local" {
  src = data.external_schema.gorm.url
  dev = "docker://postgis/postgis/18-3.6/dev?search_path=public"

  migration {
    dir = "file://migrations"
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
