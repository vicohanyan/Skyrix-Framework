// Command schemaloader prints the desired database schema of the GORM models
// as SQL for Atlas (see atlas.hcl). Add every model that is managed by SQL
// migrations to the list below.
package main

import (
	"fmt"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"

	exampleEntity "gitlab.com/skyrix-lib/template/internal/domain/example/entity"
)

func main() {
	statements, err := gormschema.New("postgres").Load(
		&exampleEntity.Task{},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load gorm schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(statements)
}
