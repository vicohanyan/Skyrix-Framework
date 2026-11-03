// Command schemaloader prints the desired database schema of the GORM models
// as SQL for db migrate:diff. Add every model that is managed by SQL
// migrations to the list below.
package main

import (
	"fmt"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"

	tenantEntity "gitlab.com/skyrix-lib/tenant/entity"
)

func main() {
	statements, err := gormschema.New("postgres").Load(
		&tenantEntity.Tenant{},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load gorm schema: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(statements)
}
