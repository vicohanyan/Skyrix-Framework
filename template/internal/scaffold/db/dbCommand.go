package db

import "github.com/spf13/cobra"

type DBCommand struct {
	AutoMigrate *DBAutoMigrateCommand
	Seed        *DBSeedCommand
}

func NewDBCommand(
	autoMigrate *DBAutoMigrateCommand,
	seed *DBSeedCommand,
) *DBCommand {
	return &DBCommand{
		AutoMigrate: autoMigrate,
		Seed:        seed,
	}
}

func (c *DBCommand) ToCobraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage the application database",
	}

	cmd.AddCommand(
		c.AutoMigrate.ToCobraCommand(),
		c.Seed.ToCobraCommand(),
	)

	return cmd
}
