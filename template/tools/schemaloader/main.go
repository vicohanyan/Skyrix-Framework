// Command schemaloader prints the desired database schema of the GORM models
// as SQL for db migrate:diff. Add every model that is managed by SQL
// migrations to the list below. The Template lists none; to run the example
// domain against a database, add &exampleEntity.Task{} (package
// gitlab.com/skyrix-lib/template/internal/domain/example/entity) and run
// "cobra db migrate:diff create_example_tasks".
package main

import (
	"fmt"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"
)

func main() {
	statements, err := gormschema.New("postgres").Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load gorm schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(statements)
}
