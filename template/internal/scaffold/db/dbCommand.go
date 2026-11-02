package db

import (
	"gitlab.com/skyrix-lib/framework/database/migrate/migratecmd"

	"github.com/spf13/cobra"
)

// DBCommand is the application's db command group: the framework SQL migration
// commands plus the application's development AutoMigrate and seed commands.
type DBCommand struct {
	AutoMigrate *DBAutoMigrateCommand
	Seed        *DBSeedCommand
	Migrations  *migratecmd.Commands
}

func NewDBCommand(
	autoMigrate *DBAutoMigrateCommand,
	seed *DBSeedCommand,
	migrations *migratecmd.Commands,
) *DBCommand {
	return &DBCommand{
		AutoMigrate: autoMigrate,
		Seed:        seed,
		Migrations:  migrations,
	}
}

func (c *DBCommand) ToCobraCommand() *cobra.Command {
	cmd := migratecmd.NewDBGroup()
	cmd.AddCommand(c.Migrations.CobraCommands()...)
	cmd.AddCommand(
		c.AutoMigrate.ToCobraCommand(),
		c.Seed.ToCobraCommand(),
	)
	return cmd
}
