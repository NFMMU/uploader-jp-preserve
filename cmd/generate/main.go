// Command generate rebuilds GORM model/query code from the live PostgreSQL schema.
// Atlas schema + versioned migrations remain the only source of truth for DDL.
package main

import (
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required; apply Atlas migrations before generating code")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	generator := gen.NewGenerator(gen.Config{
		OutPath:           "./internal/gen/query",
		ModelPkgPath:      "./internal/gen/model",
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:     true,
		FieldWithTypeTag:  true,
		FieldWithIndexTag: true,
	})
	generator.UseDB(db)
	generator.GenerateAllTable()
	generator.Execute()
	log.Println("GORM models and query code regenerated")
}
